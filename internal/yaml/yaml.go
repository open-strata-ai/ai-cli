// Package yaml implements a small, dependency-free YAML (subset) codec
// sufficient for OpenStrata manifests and CLI config files.
//
// Supported subset (block style):
//   - nested mappings (2-space indent)
//   - scalar values: string / bool / int / int64 / float64 / nil
//   - scalar sequences ("- item")
//   - struct field tags `yaml:"name"` and `yaml:"name,omitempty"`
//
// This intentionally does NOT implement the full YAML spec; it covers only the
// shapes used by openstrata.yaml / config.yaml so the CLI stays stdlib-only and
// offline-verifiable.
package yaml

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Marshal
// ---------------------------------------------------------------------------

// Marshal serializes v (struct / map / slice / scalar) into YAML block text.
func Marshal(v any) ([]byte, error) {
	var b strings.Builder
	if err := marshalValue(&b, reflect.ValueOf(v), 0); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func marshalValue(b *strings.Builder, v reflect.Value, indent int) error {
	v = unwrap(v)
	switch v.Kind() {
	case reflect.Struct:
		return marshalStruct(b, v, indent)
	case reflect.Map:
		return marshalMap(b, v, indent)
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 && v.Type().Kind() == reflect.Slice {
			// represent empty slice as empty block
			return nil
		}
		return marshalSlice(b, v, indent)
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			b.WriteString("null\n")
			return nil
		}
		return marshalValue(b, v.Elem(), indent)
	default:
		b.WriteString(scalarString(v) + "\n")
		return nil
	}
}

func marshalStruct(b *strings.Builder, v reflect.Value, indent int) error {
	t := v.Type()
	pad := strings.Repeat("  ", indent)
	fields := make([]fieldInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		name, omit := yamlTag(sf)
		if name == "-" {
			continue
		}
		fv := v.Field(i)
		if omit && isZero(fv) {
			continue
		}
		fields = append(fields, fieldInfo{name: name, val: fv})
	}
	for _, f := range fields {
		b.WriteString(pad + f.name + ":")
		if f.val.Kind() == reflect.Struct ||
			(f.val.Kind() == reflect.Map && f.val.Len() > 0) ||
			((f.val.Kind() == reflect.Slice || f.val.Kind() == reflect.Array) && f.val.Len() > 0) {
			b.WriteString("\n")
			if err := marshalValue(b, f.val, indent+1); err != nil {
				return err
			}
		} else if f.val.Kind() == reflect.Map && f.val.Len() == 0 {
			b.WriteString(" {}\n")
		} else if (f.val.Kind() == reflect.Slice || f.val.Kind() == reflect.Array) && f.val.Len() == 0 {
			b.WriteString(" []\n")
		} else {
			b.WriteString(" " + scalarString(unwrap(f.val)) + "\n")
		}
	}
	return nil
}

func marshalMap(b *strings.Builder, v reflect.Value, indent int) error {
	pad := strings.Repeat("  ", indent)
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%v", keys[i].Interface()) < fmt.Sprintf("%v", keys[j].Interface())
	})
	for _, k := range keys {
		val := v.MapIndex(k)
		b.WriteString(pad + fmt.Sprintf("%v", k.Interface()) + ":")
		if val.Kind() == reflect.Struct ||
			(val.Kind() == reflect.Map && val.Len() > 0) ||
			((val.Kind() == reflect.Slice || val.Kind() == reflect.Array) && val.Len() > 0) {
			b.WriteString("\n")
			if err := marshalValue(b, val, indent+1); err != nil {
				return err
			}
		} else {
			b.WriteString(" " + scalarString(unwrap(val)) + "\n")
		}
	}
	return nil
}

func marshalSlice(b *strings.Builder, v reflect.Value, indent int) error {
	pad := strings.Repeat("  ", indent)
	for i := 0; i < v.Len(); i++ {
		elem := unwrap(v.Index(i))
		b.WriteString(pad + "-")
		if elem.Kind() == reflect.Struct || (elem.Kind() == reflect.Map && elem.Len() > 0) {
			// inline first key handling: emit nested on following lines
			b.WriteString("\n")
			if err := marshalValue(b, elem, indent+1); err != nil {
				return err
			}
		} else {
			b.WriteString(" " + scalarString(elem) + "\n")
		}
	}
	return nil
}

func scalarString(v reflect.Value) string {
	v = unwrap(v)
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if s == "" {
			return `""`
		}
		if needsQuote(s) {
			return strconv.Quote(s)
		}
		return s
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Invalid:
		return "null"
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, ":#{}[],&*!|>%@`\"'") {
		return true
	}
	if s != strings.TrimSpace(s) {
		return true
	}
	switch s {
	case "true", "false", "null", "yes", "no", "on", "off":
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Unmarshal
// ---------------------------------------------------------------------------

// Unmarshal parses YAML block text into v (pointer to struct / map / slice).
func Unmarshal(data []byte, v any) error {
	root, err := parse(string(data))
	if err != nil {
		return err
	}
	return assign(reflect.ValueOf(v), root)
}

// node is the intermediate representation of parsed YAML.
type node struct {
	kind   nodeKind
	m      map[string]any // mapping
	s      []any          // sequence
	scalar string         // scalar value (raw)
}

type nodeKind int

const (
	kindScalar nodeKind = iota
	kindMap
	kindSeq
)

func parse(s string) (any, error) {
	lines := strings.Split(s, "\n")
	type frame struct {
		indent int
		n      *node
		key    string // key under which n is stored in its parent ("" for root)
	}
	var stack []*frame
	root := &node{kind: kindMap, m: map[string]any{}}
	stack = append(stack, &frame{indent: -1, n: root})

	for no, raw := range lines {
		line := strings.TrimRight(raw, " \r\t")
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		// Determine key/value or sequence item.
		key, val, isSeq, err := splitLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", no+1, err)
		}
		// Pop stack to the correct parent (strictly less indent).
		for len(stack) > 1 && indent <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}
		top := stack[len(stack)-1]
		parent := top.n

		if isSeq {
			if parent.kind == kindSeq {
				appendSeq(parent, val)
				continue
			}
			// parent is a map node produced by `parentKey:`; transform it
			// into a sequence stored under that key in the grandparent.
			seq := &node{kind: kindSeq, s: []any{}}
			if len(stack) >= 2 {
				if gp := stack[len(stack)-2].n; gp.kind == kindMap {
					gp.m[top.key] = seq
				}
			}
			top.n = seq
			appendSeq(seq, val)
			continue
		}

		if val == "" {
			// nested map begins (pending children: map or seq)
			child := &node{kind: kindMap, m: map[string]any{}}
			parent.m[key] = child
			stack = append(stack, &frame{indent: indent, n: child, key: key})
			continue
		}
		parent.m[key] = val
	}
	return normalize(root), nil
}

// appendSeq adds a sequence item (scalar or nested map) to a sequence node.
func appendSeq(n *node, val string) {
	if val == "" {
		n.s = append(n.s, &node{kind: kindMap, m: map[string]any{}})
	} else {
		n.s = append(n.s, val)
	}
}

// normalize converts the internal node tree into plain Go values
// (map[string]any / []any / string) so the assign step sees concrete types.
func normalize(v any) any {
	switch x := v.(type) {
	case *node:
		switch x.kind {
		case kindMap:
			m := make(map[string]any, len(x.m))
			for k, vv := range x.m {
				m[k] = normalize(vv)
			}
			return m
		case kindSeq:
			s := make([]any, 0, len(x.s))
			for _, vv := range x.s {
				s = append(s, normalize(vv))
			}
			return s
		default:
			return x.scalar
		}
	case string:
		return x
	default:
		return v
	}
}

// splitLine parses a single YAML line into (key, value, isSequence, err).
// For "- item" it returns (key="", val="item", isSeq=true).
// For "key:" or "key: value" it returns (key, val, false, nil).
func splitLine(line string) (key, val string, isSeq bool, err error) {
	if strings.HasPrefix(line, "- ") {
		return "", strings.TrimSpace(line[2:]), true, nil
	}
	if line == "-" {
		return "", "", true, nil
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false, fmt.Errorf("expected 'key: value', got %q", line)
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	val = strings.Trim(val, `"'`)
	return key, val, false, nil
}

func assign(target reflect.Value, val any) error {
	target = unwrapPtr(target)
	switch t := target.Type(); target.Kind() {
	case reflect.Struct:
		m, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to struct", val)
		}
		return assignStruct(target, m)
	case reflect.Map:
		m, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to map", val)
		}
		return assignMap(target, m)
	case reflect.Slice:
		s, ok := val.([]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to slice", val)
		}
		return assignSlice(target, s)
	default:
		scalar, ok := val.(string)
		if !ok {
			return fmt.Errorf("cannot assign %T to %s", val, t.Kind())
		}
		return setScalar(target, scalar)
	}
}

func assignStruct(target reflect.Value, m map[string]any) error {
	t := target.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		name, _ := yamlTag(sf)
		if name == "-" {
			continue
		}
		raw, ok := m[name]
		if !ok {
			continue
		}
		if err := assign(target.Field(i), raw); err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}
	return nil
}

func assignMap(target reflect.Value, m map[string]any) error {
	if target.IsNil() {
		target.Set(reflect.MakeMap(target.Type()))
	}
	for k, v := range m {
		kv := reflect.ValueOf(k)
		nv := reflect.New(target.Type().Elem()).Elem()
		if err := assign(nv, v); err != nil {
			return err
		}
		target.SetMapIndex(kv, nv)
	}
	return nil
}

func assignSlice(target reflect.Value, s []any) error {
	out := reflect.MakeSlice(target.Type(), len(s), len(s))
	for i, v := range s {
		if err := assign(out.Index(i), v); err != nil {
			return err
		}
	}
	target.Set(out)
	return nil
}

func setScalar(target reflect.Value, raw string) error {
	switch target.Kind() {
	case reflect.String:
		target.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		target.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		target.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		target.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		target.SetFloat(f)
	case reflect.Interface:
		target.Set(reflect.ValueOf(raw))
	default:
		return fmt.Errorf("unsupported scalar kind %s", target.Kind())
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type fieldInfo struct {
	name string
	val  reflect.Value
}

func yamlTag(sf reflect.StructField) (name string, omit bool) {
	tag := sf.Tag.Get("yaml")
	if tag == "" {
		return sf.Name, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = sf.Name
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omit = true
		}
	}
	return name, omit
}

func unwrap(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

func unwrapPtr(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Map, reflect.Slice, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
