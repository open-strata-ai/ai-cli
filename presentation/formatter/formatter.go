// Package formatter renders CLI output in table / json / yaml formats
// (SPECS §7.5).
package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/internal/yaml"
)

// Formatter renders a value to bytes for terminal output.
type Formatter interface {
	Format(v any) ([]byte, error)
}

// New returns a Formatter for the given format ("table"|"json"|"yaml").
func New(format string) (Formatter, error) {
	switch format {
	case "", "table":
		return &TableFormatter{}, nil
	case "json":
		return &JSONFormatter{}, nil
	case "yaml":
		return &YAMLFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown output format %q (want table|json|yaml)", format)
	}
}

// JSONFormatter renders via encoding/json.
type JSONFormatter struct{}

func (JSONFormatter) Format(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// YAMLFormatter renders via the internal yaml codec.
type YAMLFormatter struct{}

func (YAMLFormatter) Format(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

// TableFormatter renders a friendly terminal table for known value types,
// falling back to JSON for anything else.
type TableFormatter struct{}

func (TableFormatter) Format(v any) ([]byte, error) {
	switch val := v.(type) {
	case domain.Manifest:
		return renderKV([]kv{
			{"profile", val.Profile},
			{"version", val.Version},
			{"tenant", val.Tenant},
			{"model", val.Model},
			{"enabled", fmt.Sprintf("%v", sortedKeys(val.Enabled))},
		}), nil
	case domain.EvalTaskResult:
		return renderKV([]kv{
			{"task_id", val.TaskID},
			{"status", val.Status},
			{"score", fmt.Sprintf("%.2f", val.Score)},
			{"duration", val.Duration},
		}), nil
	case []domain.ModelView:
		rows := [][]string{{"MODEL_ID", "SOURCE", "ENABLED", "HEALTH", "LATENCY"}}
		for _, m := range val {
			rows = append(rows, []string{m.ModelID, m.Source, boolStr(m.Enabled), m.Health, m.Latency})
		}
		return []byte(renderTable(rows)), nil
	case []domain.EvalTaskResult:
		rows := [][]string{{"TASK_ID", "STATUS", "SCORE", "DURATION"}}
		for _, r := range val {
			rows = append(rows, []string{r.TaskID, r.Status, fmt.Sprintf("%.2f", r.Score), r.Duration})
		}
		return []byte(renderTable(rows)), nil
	case map[string]string:
		var kvs []kv
		for k, vv := range val {
			kvs = append(kvs, kv{k, vv})
		}
		return renderKV(kvs), nil
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		return b, nil
	}
}

// ---------------------------------------------------------------------------

type kv struct{ k, v string }

func renderKV(rows []kv) []byte {
	var b strings.Builder
	w := 0
	for _, r := range rows {
		if len(r.k) > w {
			w = len(r.k)
		}
	}
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("%-*s  %s\n", w, r.k, r.v))
	}
	return []byte(b.String())
}

func renderTable(rows [][]string) string {
	cols := len(rows[0])
	widths := make([]int, cols)
	for _, r := range rows {
		for i, c := range r {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	var b strings.Builder
	for ri, r := range rows {
		for i, c := range r {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(fmt.Sprintf("%-*s", widths[i], c))
		}
		b.WriteString("\n")
		if ri == 0 {
			for i, w := range widths {
				if i > 0 {
					b.WriteString("  ")
				}
				b.WriteString(strings.Repeat("-", w))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
