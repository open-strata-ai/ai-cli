# ai-cli · Interface/data/deployment specifications

> Corresponding design documents §7 (API/CLI), §8 (data model and storage), §11 (configuration and deployment)

---

## §7 CLI command surface

### 7.1 Complete command tree (Cobra)

```
aictl
├── init      --profile <p> --model <m> [--tenant <t>]
├── up        [--profile <p>] [--detach]
├── plan      --enable <cap>... [--tenant <t>]
├── apply     --plan <checksum>
├── rollback  --component <name> [--revision <rev>]
├── model     list | enable <id> | disable <id>
├── app       deploy <spec.yaml> | logs <app> | port-forward <app>
├── eval      submit <task.yaml> | status <id> | results <id>
├── config    get <key> | set <key> <val> | edit
├── debug     [--local] [--verbose]
├── version
├── login     [--tenant <t>]
└── logout
```

### 7.2 Command details

| Command | Flags | Description |
|------|-------|------|
| `init` | `--profile`, `--model`, `--tenant`, `--dry-run` | Generate openstrata.yaml |
| `up` | `--profile`, `--detach`, `--timeout` | Pull up platform core components |
| `plan` | `--enable`, `--tenant`, `--output` | Preview assembly plan |
| `apply` | `--plan`, `--wait` | Apply assembly plan |
| `rollback` | `--component`, `--revision`, `--force` | Rollback component |
| `model list` | `--enabled`, `--source`, `--output` | List model suppliers |
| `model enable` | `<model_id>` | Enable model |
| `model disable` | `<model_id>` | Disable model |
| `app deploy` | `<spec.yaml>`, `--wait` | Deploy Agent application |
| `app logs` | `<app>`, `--follow`, `--tail` | View application logs |
| `app port-forward` | `<app>`, `--port`, `--local-port` | Port forwarding |
| `eval submit` | `<task.yaml>`, `--wait` | Submit evaluation task |
| `eval status` | `<id>`, `--watch` | View evaluation progress |
| `eval results` | `<id>`, `--output` | View evaluation results |
| `config get` | `<key>`, `--output` | Read configuration items |
| `config set` | `<key>` `--value <val>` | Set configuration items |
| `config edit` | — | Open the editor to modify openstrata.yaml |
| `debug` | `--local`, `--verbose`, `--profile` | Local minimal runtime |
| `version` | `--json` | Print version information |
| `login` | `--tenant`, `--endpoint` | Keycloak OIDC login |
| `logout` | — | Clear local token |

### 7.3 Exit code

| Code | Meaning | Trigger condition |
|----|------|----------|
| 0 | Successful | Completed normally |
| 1 | General errors | Network errors, server exceptions |
| 2 | Configuration/parameter error | Illegal profile, invalid key |
| 3 | The platform is not ready | Timeout after up and not Ready |
| 4 | Assembly conflict | resolver returns Conflict |
| 5 | Authentication failed | Token expired or invalid |

### 7.4 Global Flags

| Flag | Description | Default value |
|------|------|--------|
| `--profile` | Specify profile | starter (or value from last init) |
| `--endpoint` | Platform control plane address | `http://localhost:8080` |
| `--output` | Output format: table/json/yaml | table |
| `--verbose` | Print detailed logs | false |
| `--no-color` | Disable color output | false |
| `--config` | Specify the configuration file path | `./openstrata.yaml` |

### 7.5 Output format adaptation

| Format | Command Example | Description |
|------|----------|------|
| table | `aictl model list` | Terminal-friendly table (default) |
| json | `aictl model list --output json` | Structured, pipelineable |
| yaml | `aictl config get enabled --output yaml` | editable |

---

## §8 Data Model

### 8.1 Local status directory: `~/.openstrata/`

```
~/.openstrata/
├── config.yaml              #CLI local configuration
│   ├── defaultProfile       #default profile
│   ├── platform.endpoint    #Platform control plane address
│   ├── output               #Output format (table/json/yaml)
│   └── metaRepo.profilesPath# profile local cache path
│
├── state.json               #runtime status
│   ├── currentProfile       #current profile
│   ├── lastChecksum         #Recent Plan checksum
│   └── lastUpTimestamp      #Last up time
│
├── tokens/
│   └── jwt.enc              #AES-GCM encrypted JWT Token
│
└── cache/
    └── profiles/
        ├── starter.yaml     #Local cache profile skeleton
        ├── standard.yaml
        ├── advanced.yaml
        └── full.yaml
```

### 8.2 Remote status (CLI no authoritative data)

CLI does not hold persistent business data, and all authoritative status is on the corresponding server:

| data type | authority holder | storage |
|----------|------------|------|
| assembly plan | ai-dependency-resolver | PostgreSQL (assembly_plan table) |
| Deployment status | ai-provisioning-engine | PostgreSQL (provisioning_record table) |
| Agent application | ai-platform-api | PostgreSQL |
| Model list | ai-gateway-core | Gateway configuration |
| Evaluation tasks | ai-eval-service | PostgreSQL |
| Platform Configuration | Portal/Manifest | openstrata.yaml (GitOps) |

### 8.3 schema verification rules (PlatformManifest)

```go
//openstrata.yaml structure
type PlatformManifest struct {
    Profile   string            `yaml:"profile" validate:"oneof=starter standard advanced full"`
    Version   string            `yaml:"version" validate:"semver"`
    Tenant    string            `yaml:"tenant" validate:"required,alphanum"`
    Model     string            `yaml:"model" validate:"required"`
    Enabled   map[string]bool   `yaml:"enabled" validate:"keys,valid-capability"`
    //... extension fields
}
```

### 8.4 CLI local configuration Schema

```yaml
# ~/.openstrata/config.yaml
cli:
  defaultProfile: starter     #default profile
  metaRepo:
    profilesPath: ~/.openstrata/cache/profiles
  platform:
    endpoint: http://localhost:8080
  output: table               # table | json | yaml
```

---

## §11 Deployment/Distribution

### 11.1 Distribution form

Single binary, non-K8s workload, no probes, no replicas.

| Mode | Command | Description |
|------|------|------|
| Local development | `go run ./cmd/aictl` | Source code running |
| Go install | `go install github.com/openstrata/ai-cli/cmd/aictl@v1.4.0` | Package management installation |
| Pre-compiled | CI produces multi-platform binaries | Linux/macOS/Windows (amd64/arm64) |
| Package Manager | `brew install openstrata/aictl` / `apt install aictl` | Future extensions |

### 11.2 Build configuration

```makefile
# Makefile
VERSION := $(shell git describe --tags --always)
LDFLAGS := -X main.Version=$(VERSION) -X main.PlatformVersion=v1.4.0

build:
    CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/aictl ./cmd/aictl

cross-build:
    GOOS=linux   GOARCH=amd64 go build -o bin/aictl-linux-amd64   ./cmd/aictl
    GOOS=linux   GOARCH=arm64 go build -o bin/aictl-linux-arm64   ./cmd/aictl
    GOOS=darwin  GOARCH=amd64 go build -o bin/aictl-darwin-amd64 ./cmd/aictl
    GOOS=darwin  GOARCH=arm64 go build -o bin/aictl-darwin-arm64 ./cmd/aictl
    GOOS=windows GOARCH=amd64 go build -o bin/aictl-windows-amd64.exe ./cmd/aictl
```

### 11.3 version alignment

`aictl version` output example:

```
$ aictl version
aictl version:  v1.4.0
platform:       v1.4.0
spi versions:
  Gateway:      1.2.0
  LLMProvider:  1.0.0
  Cache:        1.0.0
  Tracing:      1.0.0
build:          2026-07-17_abc1234
go version:     go1.22.0
```

```json
$ aictl version --json
{
  "cli_version": "v1.4.0",
  "platform_version": "v1.4.0",
  "spi_versions": {
    "Gateway": "1.2.0",
    "LLMProvider": "1.0.0",
    "Cache": "1.0.0",
    "Tracing": "1.0.0"
  },
  "build": "2026-07-17_abc1234",
  "go_version": "go1.22.0"
}
```

### 11.4 Local configuration file

```yaml
# Main repository infrastructure/config/config.yaml local
cli:
  defaultProfile: starter
  metaRepo:
    profilesPath: openstrata-meta/profiles
  platform:
    endpoint: http://localhost:8080
  output: table             # table|json|yaml
  timeout:
    up: 300                 #up wait timeout (seconds)
    ready: 30               #Component Ready timeout (seconds)
    request: 10             #API request timeout (seconds)
  debug:
    portRange: 8080-8090    #Local port forwarding range
```

### 11.5 Configuration key list

| Configuration Key | Default Value | Description |
|--------|--------|------|
| `cli.defaultProfile` | `starter` | Default profile |
| `cli.metaRepo.profilesPath` | `openstrata-meta/profiles` | Profile source directory |
| `cli.platform.endpoint` | `http://localhost:8080` | Platform control plane |
| `cli.output` | `table` | Output format |
| `cli.timeout.up` | `300` | up timeout seconds |
| `cli.timeout.ready` | `30` | Component ready timeout seconds |
| `cli.timeout.request` | `10` | API request timeout |
| `cli.debug.portRange` | `8080-8090` | Local port forwarding range |

### 11.6 Linkage with the platform

| Scenarios | CLI Commands | Platform Components | Protocols |
|------|----------|----------|------|
| Boot initialization | `init` | meta repository profiles | File reading |
| One-click startup (starter) | `up --profile starter` | resolver + provisioner (Compose) | HTTP |
| One-click pull up (standard+) | `up --profile advanced` | resolver + provisioner (Helm/K8s) | HTTP |
| assembly preview | `plan` | resolver | HTTP |
| Deploy application | `apply` | provisioner | HTTP |
| Model management | `model list` | gateway | HTTP |
| Application management | `app deploy` | platform-api | HTTP |
| Reviews | `eval submit` | eval-service | HTTP |
| Configuration management | `config set` | — | Local files |

### 11.7 Environment variables

| Variable | Description | Example |
|------|------|------|
| `OPENSTRATA_ENDPOINT` | Platform control plane address | `http://localhost:8080` |
| `OPENSTRATA_PROFILE` | Default profile | `starter` |
| `OPENSTRATA_TOKEN` | API Token (highest priority) | `eyJ...` |
| `OPENSTRATA_CONFIG` | Configuration file path | `./openstrata.yaml` |
| `OPENSTRATA_OUTPUT` | Output format | `json` |
| `OPENSTRATA_NO_COLOR` | Disable color output | `true` |

---

> For the core interface and package structure, see [arch/ARCH.md](../arch/ARCH.md)
> For algorithm/concurrency/safety rules, see [skills/SKILLS.md](../skills/SKILLS.md)
> For the complete process, see [design/DESIGN.md §4](../design/DESIGN.md#4-Processing Pipeline--Request Path)
