# ai-cli · Architecture documentation

> Corresponding design documents §1 (positioning and boundaries), §2 (responsibility list), §3 (core abstraction and interface), §6 (external adapter)
> Platform version v1.0.0 | Domain: developer-tooling | Required: core

---

## §1 Positioning

### 1.1 Positioning in one sentence

`ai-cli` (binary name `aictl`) is OpenStrata's unified command line entry for **developers and automation**. It converges "local development/debugging" and "interaction with the platform (assembly, deployment, evaluation, configuration)" into one command plane, shielding upper-level users from all underlying operations except the boot portal.

### 1.2 The only problem solved

Let developers use one command to complete "launch the platform from 0 → manage model/application → run evaluation → change configuration → trigger automatic upgrade" without having to manually spell Helm Values ​​or understand the nine-layer architecture.

### 1.3 System location

```
Developer CLI
  │ aictl <subcommand>
  ▼
ai-cli (Cobra command tree)
  │ PlatformClient (field Port)
  ├──→ ai-dependency-resolver  (plan)
  ├──→ ai-provisioning-engine  (up/apply/rollback)
  ├──→ ai-platform-api        (app deploy/logs)
  ├──→ ai-gateway-core         (model list/enable)
  └──→ ai-eval-service         (eval submit/status)
```

### 1.4 Boundary declaration

- **Input**: User command (Cobra subcommand + flags) + `openstrata.yaml` (local Manifest)
- **Out**: terminal output (table/JSON/YAML) + exit code
- **Not responsible**: dependency resolution (delegated resolver); deployment execution (delegated provisioner); model runtime reasoning (delegated gateway)
- **Form**: Single binary, non-K8s workload

### 1.5 Relationship with other Go components

| Components | Relationships | Description |
|------|------|------|
| ai-dependency-resolver | Downstream call | `plan` subcommand pass-through |
| ai-provisioning-engine | Downstream call | `up/apply/rollback` subcommand pass-through |
| ai-platform-api | Downstream call | `app deploy/logs/port-forward` |
| ai-gateway-core | Downstream call | `model list/enable/disable` |
| ai-eval-service | Downstream call | `eval submit/status` |
| ai-sdk-go | Peer entry | SDK embedded in host application; CLI independent binary |

### 1.6 Required

**core** (§4.1.3). There are three types of access methods in parallel with low-code workbench (business personnel) and SDK (code writing). Always distributed with the platform.

---

## §2 Responsibilities

| # | Responsibilities | Required | Description |
|---|------|------|------|
| R1 | Guided initialization | core | `aictl init --profile starter --model qwen-cloud` |
| R2 | One-click pull up | core | `aictl up`: Compose/K8s pull up core components |
| R3 | Assembly arrangement pass-through | core | `plan`/`apply`/`rollback` → resolver/provisioner |
| R4 | Model management | core | List/enable/disable model suppliers (docking gateway) |
| R5 | Application deployment | core | Deploy/debug Agent (connected to platform-api) |
| R6 | Evaluation tasks | optional | Submit/query evaluation tasks (connected to eval-service) |
| R7 | Configuration management | core | Read and write PlatformManifest (openstrata.yaml) |
| R8 | Local debugging | core | Minimum local runtime, port forwarding, log tracking |

### Responsibility interaction matrix

```
  aictl <subcommand>
       │
  ┌────┴────────────────────────────────┐
  │                                     │
  │  R7 Configuration（openstrata.yaml）          │
  │  ├── config get/set/edit            │
  │  └── Local state management (~/.openstrata/)   │
  │                                     │
  │  R1 initialization → R2 pull up                 │
  │  init ──→ up (Compose/K8s)          │
  │                                     │
  │  R3 Assembly pass-through                         │
  │  plan → resolver                    │
  │  apply/rollback → provisioner        │
  │                                     │
  │  R4 Model → gateway                   │
  │  R5 application → platform-api              │
  │  R6 Review → eval-service (optional)   │
  │                                     │
  │  R8 debug (port-forward/logs/local)   │
  └─────────────────────────────────────┘
```

---

## §3 Core interface

### 3.1 Domain layer type definition (`domain/`)

```go
package domain

import "context"

//CLIContext aggregates the context of this call
type CLIContext struct {
    Profile  string      // starter|standard|advanced|full
    Manifest ManifestRef //openstrata.yaml reference
    Endpoint string      //Platform control plane address (local or remote)
}

//ManifestRef local manifest file reference
type ManifestRef struct {
    Path    string //absolute path
    Parsed  Manifest
}

//PlatformManifest view after manifest resolution
type Manifest struct {
    Profile   string            `yaml:"profile"`
    Version   string            `yaml:"version"`
    Tenant    string            `yaml:"tenant"`
    Enabled   map[string]bool   `yaml:"enabled"`
    Model     string            `yaml:"model"`
}

//ModelView model information (from gateway)
type ModelView struct {
    ModelID string `json:"model_id"`
    Source  string `json:"source"`
    Enabled bool   `json:"enabled"`
    Health  string `json:"health"`
    Latency string `json:"latency_ms"`
}

//EvalTaskResult evaluation task summary
type EvalTaskResult struct {
    TaskID   string `json:"task_id"`
    Status   string `json:"status"` // pending|running|done|failed
    Score    float64 `json:"score"`
    Duration string `json:"duration"`
}
```

### 3.2 Domain Port (decoupling point)

```go
//PlatformClient is the core port for interacting with the platform
//The anti-corrosion layer is implemented in infrastructure/adapter/
type PlatformClient interface {
    //Initialization and pull-up
    Init(ctx context.Context, profile, model string) error
    Up(ctx context.Context, profile string) error

    //Assembly arrangement (pass-through resolver/provisioner)
    Plan(ctx context.Context, enable []string, tenant string) (string, error)   // returns checksum
    Apply(ctx context.Context, checksum string) error
    Rollback(ctx context.Context, component string) error

    //Model management
    ListModels(ctx context.Context) ([]ModelView, error)
    EnableModel(ctx context.Context, modelID string) error
    DisableModel(ctx context.Context, modelID string) error

    //Application deployment
    DeployApp(ctx context.Context, specPath string) error
    AppLogs(ctx context.Context, appName string) (io.ReadCloser, error)

    //Review
    RunEval(ctx context.Context, taskPath string) (string, error)     // returns taskID
    EvalStatus(ctx context.Context, taskID string) (EvalTaskResult, error)

    //Configuration management
    GetConfig(ctx context.Context, key string) (string, error)
    SetConfig(ctx context.Context, key, val string) error

    //debug
    PortForward(ctx context.Context, appName string, port int) error
}
```

### 3.3 Package structure (DDD four layers)

```
cmd/aictl/                    #Cobra entrance
├── root.go                   #root command
├── init.go                   #init subcommand
├── up.go                     #up subcommand
├── plan.go                   #plan subcommand
├── apply.go                  #apply subcommand
├── rollback.go               #rollback subcommand
├── model.go                  #model subcommand group
├── app.go                    #app subcommand group
├── eval.go                   #eval subcommand group
├── config.go                 #config subcommand group
├── debug.go                  #debug subcommand group
├── version.go                #version subcommand
│
├── domain/
│   ├── model.go              # CLIContext, Manifest, ModelView
│   ├── port.go               # PlatformClient interface
│   └── service.go            #Local logic (profile merging, schema verification)
│
├── application/
│   └── usecase/
│       ├── init.go           #init process orchestration
│       ├── up.go             #up process orchestration
│       └── plan.go           #plan pass-through arrangement
│
├── infrastructure/
│   ├── adapter/
│   │   ├── platform_client.go   #HTTP/gRPC PlatformClient implementation
│   │   └── gateway_client.go    #OpenAI-compatible gateway client
│   ├── config/
│   │   └── config.yaml          # cli.defaultProfile, metaRepo, output
│   └── state/
│       └── local_state.go       #~/.openstrata/ state management
│
└── presentation/
    └── formatter/
        ├── table.go             #Table output
        ├── json.go              #JSON output
        └── yaml.go              #YAML output
```

### 3.4 Cobra command organization

```go
// root.go
var rootCmd = &cobra.Command{
    Use:   "aictl",
    Short: "OpenStrata CLI - Unified developer command line tool",
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        //Load configuration, parse manifest, inject CLIContext
    },
}

//Subcommand registration
func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(upCmd)
    rootCmd.AddCommand(planCmd)
    // ...
}
```

---

## §6 Adapter

### 6.1 SPI Adapter Matrix

| SPI Ports | Roles | External Components | Default/Alternate | Adapter |
|----------|------|----------|-----------|---------|
| PlatformClient | Caller | ai-dependency-resolver / ai-provisioning-engine / ai-platform-api / ai-gateway-core | ✅ Unique | HTTP/gRPC Client Adapter |
| Gateway (1.0.0) | Caller | Higress (core, data plane) | ✅ Unique | GatewayClient (OpenAI-compatible) |
| LLMProvider (1.0.0) | Indirect | Each model supplier | ✅ Unique | Forwarded via gateway |
| Cache (1.0.0) | Consumer | Redis (core) | ✅ Unique | Local state/cache |
| Tracing (1.0.0) | Consumer | OTel (core) | ✅ Unique | CLI operation trace |

### 6.2 Anti-corrosion layer design

The CLI itself** has no runtime OSS dependencies**; all capabilities are accessed through the anti-corrosion layer client Adapter:

```go
//PlatformClientImpl aggregates multiple remote services into a unified client
type PlatformClientImpl struct {
    resolverAddr    string // ai-dependency-resolver endpoint
    provisionerAddr string // ai-provisioning-engine endpoint
    platformAddr    string // ai-platform-api endpoint
    gatewayAddr     string // ai-gateway-core endpoint
    evalAddr        string // ai-eval-service endpoint

    httpClient *http.Client //Reuse connection pool
}
```

```go
//GatewayClient OpenAI-compatible gateway client
type GatewayClient struct {
    baseURL string
    apiKey  string
}
```

### 6.3 version alignment

CLI and platform version alignment rules:

| CLI version | Platform version | SPI version |
|----------|----------|----------|
| v1.0.0 | v1.0.0 | Gateway:1.0.0, LLMProvider:1.0.0 |
| `aictl version` outputs three items: CLI version + platform version + each SPI version |

### 6.4 Local status management

```
~/.openstrata/
├── config.yaml           #CLI configuration (endpoint, profile, output format)
├── state.json            #Recent Plan checksum, current profile
├── tokens/               #Encrypted stored API Token (K8s Secret/Vault)
└── cache/
    └── profiles/         #Local cache profile skeleton (offline environment)
```

### 6.5 Adapter Testing Strategy

| Test Type | Coverage | Environment |
|----------|------|------|
| Single test | Command parameter parsing, profile merging, Manifest verification | Go test |
| Contract testing | GatewayClient/PlatformClient for each service API | Contract use case |
| E2E | `init → up` runs the golden path in kind/Compose | CI integration |
| Regression | `aictl version` aligned with `model list` version | bom version bump |

---

> For the complete command tree and exit codes, see [docs/SPECS.md](./SPECS.md)
> For algorithm/concurrency/safety rules, see [docs/SKILLS.md](./SKILLS.md)
> For the complete process of guided initialization and one-click startup, please refer to [docs/DESIGN.md §4](./DESIGN.md#4-Processing Pipeline--Request Path)
