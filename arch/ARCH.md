# ai-cli · 架构文档

> 对应设计文档 §1（定位与边界）、§2（职责清单）、§3（核心抽象与接口）、§6（外部适配器）
> 平台版本 v1.4.0 | 领域：developer-tooling | 必选性：core

---

## §1 定位

### 1.1 一句话定位

`ai-cli`（二进制名 `aictl`）是 OpenStrata 面向**开发者与自动化**的统一命令行入口。它把"本地开发/调试"与"与平台交互（装配、部署、评测、配置）"收敛到一个命令面，对上层用户屏蔽引导门户之外的全部底层操作。

### 1.2 解决的唯一问题

让开发者用一条命令完成"从 0 拉起平台 → 管理模型/应用 → 跑评测 → 改配置 → 触发自动升级"，无需手拼 Helm Values 或理解九层架构。

### 1.3 系统位置

```
开发者 CLI
  │ aictl <subcommand>
  ▼
ai-cli (Cobra 命令树)
  │ PlatformClient (领域 Port)
  ├──→ ai-dependency-resolver  (plan)
  ├──→ ai-provisioning-engine  (up/apply/rollback)
  ├──→ ai-platform-api        (app deploy/logs)
  ├──→ ai-gateway-core         (model list/enable)
  └──→ ai-eval-service         (eval submit/status)
```

### 1.4 边界声明

- **入**：用户命令（Cobra 子命令 + flags）+ `openstrata.yaml`（本地 Manifest）
- **出**：终端输出（table/JSON/YAML）+ 退出码
- **不负责**：依赖解析（委托 resolver）；部署执行（委托 provisioner）；模型运行时推理（委托 gateway）
- **形态**：单二进制，非 K8s 工作负载

### 1.5 与其他 Go 组件的关系

| 组件 | 关系 | 说明 |
|------|------|------|
| ai-dependency-resolver | 下游调用 | `plan` 子命令透传 |
| ai-provisioning-engine | 下游调用 | `up/apply/rollback` 子命令透传 |
| ai-platform-api | 下游调用 | `app deploy/logs/port-forward` |
| ai-gateway-core | 下游调用 | `model list/enable/disable` |
| ai-eval-service | 下游调用 | `eval submit/status` |
| ai-sdk-go | 同级入口 | SDK 嵌入宿主应用；CLI 独立二进制 |

### 1.6 必选性

**core**（§4.1.3）。与低代码工作台（业务人员）、SDK（写代码）并列三类接入方式。始终随平台分发。

---

## §2 职责

| # | 职责 | 必选 | 说明 |
|---|------|------|------|
| R1 | 引导式初始化 | core | `aictl init --profile starter --model qwen-cloud` |
| R2 | 一键拉起 | core | `aictl up`：Compose/K8s 拉起核心组件 |
| R3 | 装配编排透传 | core | `plan`/`apply`/`rollback` → resolver/provisioner |
| R4 | 模型管理 | core | 列出/启用/禁用模型供应方（对接网关） |
| R5 | 应用部署 | core | 部署/调试 Agent（对接 platform-api） |
| R6 | 评测任务 | optional | 提交/查询评测任务（对接 eval-service） |
| R7 | 配置管理 | core | 读写 PlatformManifest（openstrata.yaml） |
| R8 | 本地调试 | core | 本地起最小运行时、端口转发、日志跟踪 |

### 职责交互矩阵

```
  aictl <subcommand>
       │
  ┌────┴────────────────────────────────┐
  │                                     │
  │  R7 配置（openstrata.yaml）          │
  │  ├── config get/set/edit            │
  │  └── 本地状态管理 (~/.openstrata/)   │
  │                                     │
  │  R1 初始化 → R2 拉起                 │
  │  init ──→ up (Compose/K8s)          │
  │                                     │
  │  R3 装配透传                         │
  │  plan → resolver                    │
  │  apply/rollback → provisioner        │
  │                                     │
  │  R4 模型 → gateway                   │
  │  R5 应用 → platform-api              │
  │  R6 评测 → eval-service (optional)   │
  │                                     │
  │  R8 调试 (port-forward/logs/local)   │
  └─────────────────────────────────────┘
```

---

## §3 核心接口

### 3.1 领域层类型定义（`domain/`）

```go
package domain

import "context"

// CLIContext 聚合本次调用的上下文
type CLIContext struct {
    Profile  string      // starter|standard|advanced|full
    Manifest ManifestRef // openstrata.yaml 引用
    Endpoint string      // 平台控制面地址（本地或远程）
}

// ManifestRef 本地 Manifest 文件引用
type ManifestRef struct {
    Path    string // 绝对路径
    Parsed  Manifest
}

// Manifest 解析后的 PlatformManifest 视图
type Manifest struct {
    Profile   string            `yaml:"profile"`
    Version   string            `yaml:"version"`
    Tenant    string            `yaml:"tenant"`
    Enabled   map[string]bool   `yaml:"enabled"`
    Model     string            `yaml:"model"`
}

// ModelView 模型信息（来自 gateway）
type ModelView struct {
    ModelID string `json:"model_id"`
    Source  string `json:"source"`
    Enabled bool   `json:"enabled"`
    Health  string `json:"health"`
    Latency string `json:"latency_ms"`
}

// EvalTaskResult 评测任务摘要
type EvalTaskResult struct {
    TaskID   string `json:"task_id"`
    Status   string `json:"status"` // pending|running|done|failed
    Score    float64 `json:"score"`
    Duration string `json:"duration"`
}
```

### 3.2 领域 Port（解耦点）

```go
// PlatformClient 是与平台交互的核心端口
// 防腐层实现在 infrastructure/adapter/
type PlatformClient interface {
    // 初始化与拉起
    Init(ctx context.Context, profile, model string) error
    Up(ctx context.Context, profile string) error

    // 装配编排（透传 resolver/provisioner）
    Plan(ctx context.Context, enable []string, tenant string) (string, error)   // returns checksum
    Apply(ctx context.Context, checksum string) error
    Rollback(ctx context.Context, component string) error

    // 模型管理
    ListModels(ctx context.Context) ([]ModelView, error)
    EnableModel(ctx context.Context, modelID string) error
    DisableModel(ctx context.Context, modelID string) error

    // 应用部署
    DeployApp(ctx context.Context, specPath string) error
    AppLogs(ctx context.Context, appName string) (io.ReadCloser, error)

    // 评测
    RunEval(ctx context.Context, taskPath string) (string, error)     // returns taskID
    EvalStatus(ctx context.Context, taskID string) (EvalTaskResult, error)

    // 配置管理
    GetConfig(ctx context.Context, key string) (string, error)
    SetConfig(ctx context.Context, key, val string) error

    // 调试
    PortForward(ctx context.Context, appName string, port int) error
}
```

### 3.3 包结构（DDD 四层）

```
cmd/aictl/                    # Cobra 入口
├── root.go                   # 根命令
├── init.go                   # init 子命令
├── up.go                     # up 子命令
├── plan.go                   # plan 子命令
├── apply.go                  # apply 子命令
├── rollback.go               # rollback 子命令
├── model.go                  # model 子命令组
├── app.go                    # app 子命令组
├── eval.go                   # eval 子命令组
├── config.go                 # config 子命令组
├── debug.go                  # debug 子命令组
├── version.go                # version 子命令
│
├── domain/
│   ├── model.go              # CLIContext, Manifest, ModelView
│   ├── port.go               # PlatformClient interface
│   └── service.go            # 本地逻辑（profile 合并、schema 校验）
│
├── application/
│   └── usecase/
│       ├── init.go           # init 流程编排
│       ├── up.go             # up 流程编排
│       └── plan.go           # plan 透传编排
│
├── infrastructure/
│   ├── adapter/
│   │   ├── platform_client.go   # HTTP/gRPC PlatformClient 实现
│   │   └── gateway_client.go    # OpenAI-compatible 网关客户端
│   ├── config/
│   │   └── config.yaml          # cli.defaultProfile, metaRepo, output
│   └── state/
│       └── local_state.go       # ~/.openstrata/ 状态管理
│
└── presentation/
    └── formatter/
        ├── table.go             # 表格输出
        ├── json.go              # JSON 输出
        └── yaml.go              # YAML 输出
```

### 3.4 Cobra 命令组织

```go
// root.go
var rootCmd = &cobra.Command{
    Use:   "aictl",
    Short: "OpenStrata CLI - 统一的开发者命令行工具",
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        // 加载配置、解析 Manifest、注入 CLIContext
    },
}

// 子命令注册
func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(upCmd)
    rootCmd.AddCommand(planCmd)
    // ...
}
```

---

## §6 适配器

### 6.1 SPI 适配器矩阵

| SPI 端口 | 角色 | 外部组件 | 默认/备选 | Adapter |
|----------|------|----------|-----------|---------|
| PlatformClient | 调用方 | ai-dependency-resolver / ai-provisioning-engine / ai-platform-api / ai-gateway-core | ✅ 唯一 | HTTP/gRPC Client Adapter |
| Gateway (1.2.0) | 调用方 | Higress（core，数据面） | ✅ 唯一 | GatewayClient（OpenAI-compatible） |
| LLMProvider (1.0.0) | 间接 | 各模型供应方 | ✅ 唯一 | 经网关转发 |
| Cache (1.0.0) | 消费方 | Redis（core） | ✅ 唯一 | 本地状态/缓存 |
| Tracing (1.0.0) | 消费方 | OTel（core） | ✅ 唯一 | CLI 操作 trace |

### 6.2 防腐层设计

CLI 自身**无运行时 OSS 依赖**；所有能力经防腐层客户端 Adapter 访问：

```go
// PlatformClientImpl 将多个远端服务聚合为一个统一客户端
type PlatformClientImpl struct {
    resolverAddr    string // ai-dependency-resolver endpoint
    provisionerAddr string // ai-provisioning-engine endpoint
    platformAddr    string // ai-platform-api endpoint
    gatewayAddr     string // ai-gateway-core endpoint
    evalAddr        string // ai-eval-service endpoint

    httpClient *http.Client // 复用连接池
}
```

```go
// GatewayClient OpenAI-compatible 网关客户端
type GatewayClient struct {
    baseURL string
    apiKey  string
}
```

### 6.3 版本对齐

CLI 与平台版本对齐规则：

| CLI 版本 | 平台版本 | SPI 版本 |
|----------|----------|----------|
| v1.4.0 | v1.4.0 | Gateway:1.2.0, LLMProvider:1.0.0 |
| `aictl version` 输出三项：CLI 版本 + 平台版本 + 各 SPI 版本 |

### 6.4 本地状态管理

```
~/.openstrata/
├── config.yaml           # CLI 配置（endpoint, profile, output format）
├── state.json            # 最近 Plan checksum、当前 profile
├── tokens/               # 加密存储的 API Token（K8s Secret/Vault）
└── cache/
    └── profiles/         # 本地缓存 profile 骨架（离线环境）
```

### 6.5 适配器测试策略

| 测试类型 | 覆盖 | 环境 |
|----------|------|------|
| 单测 | 命令参数解析、profile 合并、Manifest 校验 | Go test |
| 契约测试 | GatewayClient/PlatformClient 对各服务 API | 契约用例 |
| E2E | `init → up` 在 kind/Compose 中跑通黄金路径 | CI 集成 |
| 回归 | `aictl version` 与 `model list` 版本对齐 | bom 版本 bump |

---

> 完整命令树及退出码参见 [specs/SPECS.md](../specs/SPECS.md)
> 算法/并发/安全规则参见 [skills/SKILLS.md](../skills/SKILLS.md)
> 引导式初始化与一键拉起的完整流程参见 [design/DESIGN.md §4](../design/DESIGN.md#4-处理流水线--请求路径)
