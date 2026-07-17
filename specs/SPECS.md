# ai-cli · 接口/数据/部署规格

> 对应设计文档 §7（API/CLI）、§8（数据模型与存储）、§11（配置与部署）

---

## §7 CLI 命令面

### 7.1 完整命令树（Cobra）

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

### 7.2 命令详情

| 命令 | Flags | 说明 |
|------|-------|------|
| `init` | `--profile`, `--model`, `--tenant`, `--dry-run` | 生成 openstrata.yaml |
| `up` | `--profile`, `--detach`, `--timeout` | 拉起平台核心组件 |
| `plan` | `--enable`, `--tenant`, `--output` | 预览装配计划 |
| `apply` | `--plan`, `--wait` | 应用装配计划 |
| `rollback` | `--component`, `--revision`, `--force` | 回滚组件 |
| `model list` | `--enabled`, `--source`, `--output` | 列出模型供应方 |
| `model enable` | `<model_id>` | 启用模型 |
| `model disable` | `<model_id>` | 禁用模型 |
| `app deploy` | `<spec.yaml>`, `--wait` | 部署 Agent 应用 |
| `app logs` | `<app>`, `--follow`, `--tail` | 查看应用日志 |
| `app port-forward` | `<app>`, `--port`, `--local-port` | 端口转发 |
| `eval submit` | `<task.yaml>`, `--wait` | 提交评测任务 |
| `eval status` | `<id>`, `--watch` | 查看评测进度 |
| `eval results` | `<id>`, `--output` | 查看评测结果 |
| `config get` | `<key>`, `--output` | 读取配置项 |
| `config set` | `<key>` `--value <val>` | 设置配置项 |
| `config edit` | — | 打开编辑器修改 openstrata.yaml |
| `debug` | `--local`, `--verbose`, `--profile` | 本地最小运行时 |
| `version` | `--json` | 打印版本信息 |
| `login` | `--tenant`, `--endpoint` | Keycloak OIDC 登录 |
| `logout` | — | 清除本地 token |

### 7.3 退出码

| 码 | 含义 | 触发条件 |
|----|------|----------|
| 0 | 成功 | 正常完成 |
| 1 | 通用错误 | 网络错误、服务端异常 |
| 2 | 配置/参数错误 | 非法 profile、无效 key |
| 3 | 平台未就绪 | up 后超时未 Ready |
| 4 | 装配冲突 | resolver 返回 Conflict |
| 5 | 鉴权失败 | Token 过期或无效 |

### 7.4 全局 Flags

| Flag | 说明 | 默认值 |
|------|------|--------|
| `--profile` | 指定 profile | starter（或上次 init 的值） |
| `--endpoint` | 平台控制面地址 | `http://localhost:8080` |
| `--output` | 输出格式：table / json / yaml | table |
| `--verbose` | 打印详细日志 | false |
| `--no-color` | 禁用彩色输出 | false |
| `--config` | 指定配置文件路径 | `./openstrata.yaml` |

### 7.5 输出格式适配

| 格式 | 命令示例 | 说明 |
|------|----------|------|
| table | `aictl model list` | 终端友好的表格（默认） |
| json | `aictl model list --output json` | 结构化，可管道处理 |
| yaml | `aictl config get enabled --output yaml` | 可编辑 |

---

## §8 数据模型

### 8.1 本地状态目录：`~/.openstrata/`

```
~/.openstrata/
├── config.yaml              # CLI 本地配置
│   ├── defaultProfile       # 默认 profile
│   ├── platform.endpoint    # 平台控制面地址
│   ├── output               # 输出格式（table/json/yaml）
│   └── metaRepo.profilesPath# profile 本地缓存路径
│
├── state.json               # 运行时状态
│   ├── currentProfile       # 当前 profile
│   ├── lastChecksum         # 最近 Plan checksum
│   └── lastUpTimestamp      # 最近 up 时间
│
├── tokens/
│   └── jwt.enc              # AES-GCM 加密的 JWT Token
│
└── cache/
    └── profiles/
        ├── starter.yaml     # 本地缓存 profile 骨架
        ├── standard.yaml
        ├── advanced.yaml
        └── full.yaml
```

### 8.2 远端状态（CLI 无权威数据）

CLI 不持有持久化业务数据，所有权威状态在对应服务端：

| 数据类型 | 权威持有者 | 存储 |
|----------|------------|------|
| 装配计划 | ai-dependency-resolver | PostgreSQL（assembly_plan 表） |
| 部署状态 | ai-provisioning-engine | PostgreSQL（provisioning_record 表） |
| Agent 应用 | ai-platform-api | PostgreSQL |
| 模型列表 | ai-gateway-core | 网关配置 |
| 评测任务 | ai-eval-service | PostgreSQL |
| 平台配置 | 门户/Manifest | openstrata.yaml（GitOps） |

### 8.3 schema 校验规则（PlatformManifest）

```go
// openstrata.yaml 结构
type PlatformManifest struct {
    Profile   string            `yaml:"profile" validate:"oneof=starter standard advanced full"`
    Version   string            `yaml:"version" validate:"semver"`
    Tenant    string            `yaml:"tenant" validate:"required,alphanum"`
    Model     string            `yaml:"model" validate:"required"`
    Enabled   map[string]bool   `yaml:"enabled" validate:"keys,valid-capability"`
    // ... 扩展字段
}
```

### 8.4 CLI 本地配置 Schema

```yaml
# ~/.openstrata/config.yaml
cli:
  defaultProfile: starter     # 默认 profile
  metaRepo:
    profilesPath: ~/.openstrata/cache/profiles
  platform:
    endpoint: http://localhost:8080
  output: table               # table | json | yaml
```

---

## §11 部署/分发

### 11.1 分发形态

单二进制，非 K8s 工作负载，无探针、无副本。

| 方式 | 命令 | 说明 |
|------|------|------|
| 本地开发 | `go run ./cmd/aictl` | 源码运行 |
| Go install | `go install github.com/openstrata/ai-cli/cmd/aictl@v1.4.0` | 包管理安装 |
| 预编译 | CI 产出多平台二进制 | Linux/macOS/Windows (amd64/arm64) |
| 包管理器 | `brew install openstrata/aictl` / `apt install aictl` | 未来扩展 |

### 11.2 构建配置

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

### 11.3 版本对齐

`aictl version` 输出示例：

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

### 11.4 本地配置文件

```yaml
# 本仓 infrastructure/config/config.yaml 局部
cli:
  defaultProfile: starter
  metaRepo:
    profilesPath: openstrata-meta/profiles
  platform:
    endpoint: http://localhost:8080
  output: table             # table|json|yaml
  timeout:
    up: 300                 # up 等待超时（秒）
    ready: 30               # 组件 Ready 超时（秒）
    request: 10             # API 请求超时（秒）
  debug:
    portRange: 8080-8090    # 本地端口转发范围
```

### 11.5 配置键清单

| 配置键 | 默认值 | 说明 |
|--------|--------|------|
| `cli.defaultProfile` | `starter` | 默认 profile |
| `cli.metaRepo.profilesPath` | `openstrata-meta/profiles` | Profile 源目录 |
| `cli.platform.endpoint` | `http://localhost:8080` | 平台控制面 |
| `cli.output` | `table` | 输出格式 |
| `cli.timeout.up` | `300` | up 超时秒数 |
| `cli.timeout.ready` | `30` | 组件就绪超时秒数 |
| `cli.timeout.request` | `10` | API 请求超时 |
| `cli.debug.portRange` | `8080-8090` | 本地端口转发范围 |

### 11.6 与平台联动

| 场景 | CLI 命令 | 平台组件 | 协议 |
|------|----------|----------|------|
| 引导初始化 | `init` | 元仓 profiles | 文件读取 |
| 一键拉起（starter）| `up --profile starter` | resolver + provisioner（Compose） | HTTP |
| 一键拉起（standard+）| `up --profile advanced` | resolver + provisioner（Helm/K8s） | HTTP |
| 装配预览 | `plan` | resolver | HTTP |
| 部署应用 | `apply` | provisioner | HTTP |
| 模型管理 | `model list` | gateway | HTTP |
| 应用管理 | `app deploy` | platform-api | HTTP |
| 评测 | `eval submit` | eval-service | HTTP |
| 配置管理 | `config set` | — | 本地文件 |

### 11.7 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `OPENSTRATA_ENDPOINT` | 平台控制面地址 | `http://localhost:8080` |
| `OPENSTRATA_PROFILE` | 默认 profile | `starter` |
| `OPENSTRATA_TOKEN` | API Token（优先级最高） | `eyJ...` |
| `OPENSTRATA_CONFIG` | 配置文件路径 | `./openstrata.yaml` |
| `OPENSTRATA_OUTPUT` | 输出格式 | `json` |
| `OPENSTRATA_NO_COLOR` | 禁用彩色输出 | `true` |

---

> 核心接口与包结构参见 [arch/ARCH.md](../arch/ARCH.md)
> 算法/并发/安全规则参见 [skills/SKILLS.md](../skills/SKILLS.md)
> 完整流程参见 [design/DESIGN.md §4](../design/DESIGN.md#4-处理流水线--请求路径)
