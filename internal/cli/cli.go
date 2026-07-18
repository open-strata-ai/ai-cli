// Package cli implements the aictl command tree and execution engine using
// only the standard library (no Cobra dependency), keeping the CLI
// offline-verifiable. Commands are thin: they parse flags, invoke an
// application use case, and render output via the formatter.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/config"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
	"github.com/open-strata-ai/ai-cli/presentation/formatter"
)

// Command is a node in the aictl command tree.
type Command struct {
	Name        string
	Short       string
	Usage       string
	Subcommands []*Command
	// Bind registers command-specific flags and returns a struct holding the
	// parsed values (passed back to Run).
	Bind func(fs *flag.FlagSet) any
	Run  func(rc *RunContext, bind any, args []string) error
}

// RunContext carries everything a command Run needs.
type RunContext struct {
	Ctx        context.Context
	Profile    string
	Endpoint   string
	Output     string
	ConfigPath string
	Verbose    bool
	NoColor    bool
	Client     domain.PlatformClient
	Service    *domain.Service
	StateDir   string
	Token      string
	Out        io.Writer
}

// Formatter builds the configured output formatter.
func (rc *RunContext) Formatter() (formatter.Formatter, error) {
	return formatter.New(rc.Output)
}

// Emit formats v and writes it to the output stream.
func (rc *RunContext) Emit(v any) error {
	f, err := rc.Formatter()
	if err != nil {
		return err
	}
	b, err := f.Format(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(rc.Out, string(b))
	return err
}

// EmitString writes a plain line.
func (rc *RunContext) EmitString(s string) error {
	_, err := fmt.Fprintln(rc.Out, s)
	return err
}

// GlobalFlags holds parsed global flags.
type GlobalFlags struct {
	profile  string
	endpoint string
	output   string
	config   string
	verbose  bool
	noColor  bool
}

// stringSlice is a repeatable string flag.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// stateDirResolver resolves the local state directory. Overridable in tests
// to avoid touching the real ~/.openstrata.
var stateDirResolver = state.DefaultDir

// Execute parses args, builds the RunContext, and dispatches to the command
// tree. It returns the process exit code.
func Execute(args []string, out io.Writer, client domain.PlatformClient) int {
	g, rest := extractGlobals(args)
	if len(rest) == 0 {
		printHelp(out)
		return 0
	}
	cfg, err := config.Load(g.config)
	if err != nil {
		fmt.Fprintln(out, "config error:", err)
		return 2
	}
	profile, endpoint, output := cfg.Effective(g.profile, g.endpoint, g.output)
	rc := &RunContext{
		Ctx:        context.Background(),
		Profile:    profile,
		Endpoint:   endpoint,
		Output:     output,
		ConfigPath: g.config,
		Verbose:    g.verbose,
		NoColor:    g.noColor,
		Client:     client,
		Service:    domain.NewService(),
		StateDir:   stateDirResolver(),
		Token:      state.ResolveToken(stateDirResolver(), ""),
		Out:        out,
	}
	if err := rc.dispatch(rest); err != nil {
		fmt.Fprintln(out, "error:", err)
		return domain.ExitCodeOf(err)
	}
	return 0
}

func (rc *RunContext) dispatch(rest []string) error {
	root := buildRoot()
	cmd := find(root.Subcommands, rest[0])
	if cmd == nil {
		return domain.ErrConfig(fmt.Sprintf("unknown command %q", rest[0]), nil)
	}
	if len(cmd.Subcommands) > 0 && len(rest) > 1 {
		sub := find(cmd.Subcommands, rest[1])
		if sub == nil {
			return domain.ErrConfig(fmt.Sprintf("unknown subcommand %q for %q", rest[1], rest[0]), nil)
		}
		return rc.runLeaf(sub, rest[2:])
	}
	return rc.runLeaf(cmd, rest[1:])
}

func (rc *RunContext) runLeaf(cmd *Command, args []string) error {
	fs := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	fs.SetOutput(rc.Out)
	var bind any
	if cmd.Bind != nil {
		bind = cmd.Bind(fs)
	}
	if err := fs.Parse(args); err != nil {
		return domain.ErrConfig(err.Error(), nil)
	}
	return cmd.Run(rc, bind, fs.Args())
}

func find(cmds []*Command, name string) *Command {
	for _, c := range cmds {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// extractGlobals pulls global flags from anywhere in the argument list,
// leaving command-specific args in place (SPECS §7.4).
func extractGlobals(args []string) (GlobalFlags, []string) {
	var g GlobalFlags
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			continue
		}
		name, val, hasVal := splitFlag(a)
		switch name {
		case "profile", "endpoint", "output", "config":
			if !hasVal {
				i++
				val = args[i]
			}
			switch name {
			case "profile":
				g.profile = val
			case "endpoint":
				g.endpoint = val
			case "output":
				g.output = val
			case "config":
				g.config = val
			}
		case "verbose":
			g.verbose = true
		case "no-color":
			g.noColor = true
		default:
			rest = append(rest, a)
		}
	}
	if g.config == "" {
		g.config = osConfigPath()
	}
	return g, rest
}

func splitFlag(a string) (name, val string, hasVal bool) {
	s := strings.TrimLeft(a, "-")
	if i := strings.Index(s, "="); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

func osConfigPath() string {
	if v := os.Getenv("OPENSTRATA_CONFIG"); v != "" {
		return v
	}
	return "./openstrata.yaml"
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "aictl - OpenStrata unified developer CLI")
	fmt.Fprintln(out, "Usage: aictl [global flags] <command> [flags]")
	fmt.Fprintln(out, "\nCommands:")
	for _, c := range buildRoot().Subcommands {
		fmt.Fprintf(out, "  %-10s %s\n", c.Name, c.Short)
	}
	fmt.Fprintln(out, "\nGlobal flags: --profile --endpoint --output --config --verbose --no-color")
}
