package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/open-strata-ai/ai-cli/application/usecase"
	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

// Version metadata surfaced by `aictl version` (SPECS §11.3).
var (
	Version         = "v1.0.0"
	PlatformVersion = "v1.0.0"
	Build           = "dev"
	spiVersions     = map[string]string{"Gateway": "1.0.0", "LLMProvider": "1.0.0", "Cache": "1.0.0", "Tracing": "1.0.0"}
)

// buildRoot returns the aictl command tree (SPECS §7.1).
func buildRoot() *Command {
	return &Command{
		Name: "aictl",
		Subcommands: []*Command{
			{
				Name:  "init",
				Short: "Generate openstrata.yaml from a profile",
				Bind: func(fs *flag.FlagSet) any {
					a := &initArgs{}
					fs.StringVar(&a.model, "model", "", "model supplier")
					fs.StringVar(&a.tenant, "tenant", "", "tenant id")
					fs.BoolVar(&a.dryRun, "dry-run", false, "print manifest without writing")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					a := bind.(*initArgs)
					uc := usecase.NewInitUseCase(rc.Service, rc.Client, rc.StateDir)
					m, err := uc.Run(rc.Ctx, rc.Profile, a.model, a.tenant, a.dryRun, rc.ConfigPath)
					if err != nil {
						return err
					}
					if a.dryRun {
						return rc.Emit(m)
					}
					return rc.EmitString(fmt.Sprintf("wrote %s (profile=%s, model=%s, tenant=%s)", rc.ConfigPath, m.Profile, m.Model, m.Tenant))
				},
			},
			{
				Name:  "up",
				Short: "Pull up platform core components",
				Bind: func(fs *flag.FlagSet) any {
					a := &upArgs{}
					fs.BoolVar(&a.detach, "detach", false, "run in background")
					return a
				},
				Run: func(rc *RunContext, _ any, _ []string) error {
					uc := usecase.NewUpUseCase(rc.Client, rc.StateDir)
					if err := uc.Run(rc.Ctx, rc.Profile); err != nil {
						return err
					}
					return rc.EmitString("up complete for profile " + rc.Profile)
				},
			},
			{
				Name:  "plan",
				Short: "Preview the assembly plan (pass-through to resolver)",
				Bind: func(fs *flag.FlagSet) any {
					a := &planArgs{}
					fs.Var(&a.enable, "enable", "capability to enable (repeatable)")
					fs.StringVar(&a.tenant, "tenant", "", "tenant id")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					a := bind.(*planArgs)
					uc := usecase.NewPlanUseCase(rc.Service, rc.Client, rc.StateDir, rc.ConfigPath)
					cs, err := uc.Run(rc.Ctx, a.enable, a.tenant)
					if err != nil {
						return err
					}
					return rc.EmitString("plan checksum: " + cs)
				},
			},
			{
				Name:  "apply",
				Short: "Apply an assembly plan",
				Bind: func(fs *flag.FlagSet) any {
					a := &applyArgs{}
					fs.StringVar(&a.plan, "plan", "", "plan checksum (defaults to last)")
					fs.BoolVar(&a.wait, "wait", false, "wait for completion")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					a := bind.(*applyArgs)
					uc := usecase.NewApplyUseCase(rc.Client, rc.StateDir)
					if err := uc.Run(rc.Ctx, a.plan); err != nil {
						return err
					}
					return rc.EmitString("applied " + a.plan)
				},
			},
			{
				Name:  "rollback",
				Short: "Roll back a component",
				Bind: func(fs *flag.FlagSet) any {
					a := &rollbackArgs{}
					fs.StringVar(&a.component, "component", "", "component to roll back")
					fs.StringVar(&a.revision, "revision", "", "target revision")
					fs.BoolVar(&a.force, "force", false, "force rollback")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					a := bind.(*rollbackArgs)
					uc := usecase.NewRollbackUseCase(rc.Client)
					return uc.Run(rc.Ctx, a.component)
				},
			},
			modelCommand(),
			appCommand(),
			evalCommand(),
			configCommand(),
			{
				Name:  "debug",
				Short: "Local minimal runtime diagnostics",
				Bind: func(fs *flag.FlagSet) any {
					a := &debugArgs{}
					fs.BoolVar(&a.local, "local", false, "local runtime")
					fs.BoolVar(&a.verbose, "verbose", false, "verbose")
					return a
				},
				Run: func(rc *RunContext, _ any, _ []string) error {
					uc := usecase.NewDebugUseCase(rc.StateDir)
					m, err := uc.Run(rc.Ctx)
					if err != nil {
						return err
					}
					return rc.Emit(m)
				},
			},
			{
				Name:  "version",
				Short: "Show version information",
				Bind: func(fs *flag.FlagSet) any {
					a := &versionArgs{}
					fs.BoolVar(&a.json, "json", false, "JSON output")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					return runVersion(rc, bind.(*versionArgs).json)
				},
			},
			{
				Name:  "login",
				Short: "Authenticate and store API token",
				Bind: func(fs *flag.FlagSet) any {
					a := &loginArgs{}
					fs.StringVar(&a.tenant, "tenant", "", "tenant id")
					fs.StringVar(&a.endpoint, "endpoint", "", "control-plane endpoint")
					fs.StringVar(&a.token, "token", "", "API token (else OPENSTRATA_TOKEN / stored)")
					return a
				},
				Run: func(rc *RunContext, bind any, _ []string) error {
					a := bind.(*loginArgs)
					tok := state.ResolveToken(rc.StateDir, a.token)
					uc := usecase.NewLoginUseCase(rc.StateDir)
					if err := uc.Run(rc.Ctx, a.tenant, tok); err != nil {
						return err
					}
					return rc.EmitString("logged in (token stored encrypted)")
				},
			},
			{
				Name:  "logout",
				Short: "Clear the local API token",
				Run: func(rc *RunContext, _ any, _ []string) error {
					uc := usecase.NewLogoutUseCase(rc.StateDir)
					if err := uc.Run(rc.Ctx); err != nil {
						return err
					}
					return rc.EmitString("logged out")
				},
			},
		},
	}
}

func modelCommand() *Command {
	return &Command{
		Name:  "model",
		Short: "Manage model suppliers (via gateway)",
		Subcommands: []*Command{
			{Name: "list", Short: "List model suppliers", Run: func(rc *RunContext, _ any, _ []string) error {
				uc := usecase.NewModelUseCase(rc.Client)
				views, err := uc.List(rc.Ctx)
				if err != nil {
					return err
				}
				return rc.Emit(views)
			}},
			{Name: "enable", Short: "Enable a model", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("model enable requires <model_id>", nil)
				}
				return usecase.NewModelUseCase(rc.Client).Enable(rc.Ctx, args[0])
			}},
			{Name: "disable", Short: "Disable a model", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("model disable requires <model_id>", nil)
				}
				return usecase.NewModelUseCase(rc.Client).Disable(rc.Ctx, args[0])
			}},
		},
	}
}

func appCommand() *Command {
	return &Command{
		Name:  "app",
		Short: "Deploy and debug Agent applications",
		Subcommands: []*Command{
			{Name: "deploy", Short: "Deploy an Agent app", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("app deploy requires <spec.yaml>", nil)
				}
				return usecase.NewAppUseCase(rc.Client).Deploy(rc.Ctx, args[0])
			}},
			{Name: "logs", Short: "Stream application logs", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("app logs requires <app>", nil)
				}
				r, err := usecase.NewAppUseCase(rc.Client).Logs(rc.Ctx, args[0])
				if err != nil {
					return err
				}
				defer r.Close()
				_, err = io.Copy(rc.Out, r)
				return err
			}},
			{Name: "port-forward", Short: "Port forward to an application",
				Bind: func(fs *flag.FlagSet) any {
					a := &portForwardArgs{}
					fs.IntVar(&a.port, "port", 0, "remote port")
					fs.IntVar(&a.localPort, "local-port", 0, "local port")
					return a
				},
				Run: func(rc *RunContext, bind any, args []string) error {
					if len(args) < 1 {
						return domain.ErrConfig("app port-forward requires <app>", nil)
					}
					return usecase.NewAppUseCase(rc.Client).PortForward(rc.Ctx, args[0], bind.(*portForwardArgs).port)
				}},
		},
	}
}

func evalCommand() *Command {
	return &Command{
		Name:  "eval",
		Short: "Run evaluation tasks (via eval-service)",
		Subcommands: []*Command{
			{Name: "submit", Short: "Submit an evaluation task", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("eval submit requires <task.yaml>", nil)
				}
				id, err := usecase.NewEvalUseCase(rc.Client).Submit(rc.Ctx, args[0])
				if err != nil {
					return err
				}
				return rc.EmitString("eval task: " + id)
			}},
			{Name: "status", Short: "Show evaluation status", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("eval status requires <id>", nil)
				}
				r, err := usecase.NewEvalUseCase(rc.Client).Status(rc.Ctx, args[0])
				if err != nil {
					return err
				}
				return rc.Emit(r)
			}},
			{Name: "results", Short: "Show evaluation results", Run: func(rc *RunContext, _ any, args []string) error {
				if len(args) < 1 {
					return domain.ErrConfig("eval results requires <id>", nil)
				}
				r, err := usecase.NewEvalUseCase(rc.Client).Results(rc.Ctx, args[0])
				if err != nil {
					return err
				}
				return rc.Emit(r)
			}},
		},
	}
}

func configCommand() *Command {
	return &Command{
		Name:  "config",
		Short: "Read/write PlatformManifest (openstrata.yaml)",
		Subcommands: []*Command{
			{Name: "get", Short: "Get a config value",
				Run: func(rc *RunContext, _ any, args []string) error {
					if len(args) < 1 {
						return domain.ErrConfig("config get requires <key>", nil)
					}
					v, err := usecase.NewConfigUseCase(rc.Service, rc.ConfigPath, rc.StateDir).Get(rc.Ctx, args[0])
					if err != nil {
						return err
					}
					return rc.EmitString(args[0] + ": " + v)
				}},
			{Name: "set", Short: "Set a config value",
				Run: func(rc *RunContext, _ any, args []string) error {
					if len(args) < 2 {
						return domain.ErrConfig("config set requires <key> <val>", nil)
					}
					if err := usecase.NewConfigUseCase(rc.Service, rc.ConfigPath, rc.StateDir).Set(rc.Ctx, args[0], args[1]); err != nil {
						return err
					}
					return rc.EmitString("set " + args[0] + "=" + args[1])
				}},
			{Name: "edit", Short: "Open editor to edit openstrata.yaml",
				Run: func(rc *RunContext, _ any, _ []string) error {
					return rc.EmitString("edit " + rc.ConfigPath + " (open in $EDITOR)")
				}},
		},
	}
}

func runVersion(rc *RunContext, asJSON bool) error {
	if asJSON {
		return rc.Emit(map[string]any{
			"cli_version":      Version,
			"platform_version": PlatformVersion,
			"build":            Build,
			"go_version":       state.GoVersion(),
		})
	}
	return rc.EmitString(fmt.Sprintf("aictl version: %s\nplatform:       %s\nbuild:          %s\ngo version:     %s",
		Version, PlatformVersion, Build, state.GoVersion()))
}

// ---- flag arg structs ----

type initArgs struct {
	model  string
	tenant string
	dryRun bool
}
type upArgs struct{ detach bool }
type planArgs struct {
	enable stringSlice
	tenant string
}
type applyArgs struct {
	plan string
	wait bool
}
type rollbackArgs struct {
	component string
	revision  string
	force     bool
}
type debugArgs struct {
	local   bool
	verbose bool
}
type versionArgs struct{ json bool }
type loginArgs struct {
	tenant   string
	endpoint string
	token    string
}
type portForwardArgs struct {
	port      int
	localPort int
}
