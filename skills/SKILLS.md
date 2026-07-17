# ai-cli · 算法/并发/安全规则

> 对应设计文档 §5（关键算法）、§9（并发与性能）、§12（可观测性/安全）

---

## §5 关键算法

### 5.1 引导式初始化（`aictl init`）

**输入**：`--profile <p> --model <m>` + 可选 `--tenant <t>`
**输出**：`openstrata.yaml`（PlatformManifest）

```
function init(profile, model, tenant):
    skeleton = loadProfile(profile)        // openstrata-meta/profiles/<p>.yaml
    defaults = skeleton.defaults           // external, optional_disabled
    merged   = deepMerge(defaults, {
        profile: profile,
        model:   model,
        tenant:  tenant || "default",
        enabled: skeleton.enabled          // profile 推荐启用列表
    })

    validateSchema(merged)                  // 对齐 §12.1 schema
    writeYAML("openstrata.yaml", merged)

    return manifestPath
```

**Schema 校验规则**：
- `profile` 必须为四档之一（starter/standard/advanced/full）
- `enabled` key 必须在 Registry 中存在
- `model` 非空（starter 至少选一个 LLM 供应方）

### 5.2 一键拉起（`aictl up`）

```
function up(profile, detach):
    renderer = selectRenderer(profile)
    // starter → Compose; standard+/advanced/full → Helm/K8s

    plan = callResolver.plan(profile)       // 调用 ai-dependency-resolver
    result = callProvisioner.apply(plan)     // 调用 ai-provisioning-engine

    if not detach:
        // 本地开发：端口转发 + 探测 Ready
        portForward(gateway, 8080:80)
        portForward(ui, 3000:80)

        err = waitForReady([
            {name: "gateway", url: "http://localhost:8080/healthz"},
            {name: "ui",      url: "http://localhost:3000"},
        ], timeout=30s)

        if err == nil:
            openBrowser("http://localhost:3000")  // 聊天 UI
```

### 5.3 装配透传

`plan`/`apply`/`rollback` 子命令**零业务逻辑**——仅收集参数、调用 PlatformClient、格式化输出：

```
function plan(enable, tenant):
    checksum = platformClient.Plan(ctx, enable, tenant)
    print("Plan checksum: " + checksum)
    return checksum

function apply(checksum):
    err = platformClient.Apply(ctx, checksum)
    if err: print("Apply failed: " + err)
    return err
```

### 5.4 配置读写

```
function configSet(key, val):
    manifest = loadManifest("openstrata.yaml")
    manifest.set(key, val)
    validateSchema(manifest)          // 写前校验
    writeYAML("openstrata.yaml", manifest)

function configGet(key):
    manifest = loadManifest("openstrata.yaml")
    return manifest.get(key)
```

### 5.5 Profile 合并策略

```
优先级（高→低）：
1. 命令行 --enable flag（显式指定）
2. openstrata.yaml 用户已有配置
3. profiles/<p>.yaml 骨架（external + optional_disabled）
4. 全局默认（bom.yaml 默认值）
```

---

## §9 并发与性能

### 9.1 执行模型

- **框架**：Cobra 单二进制，无长驻服务
- **每个命令独立进程**：发起即退出（`up --detach` 除外）
- **CLI 自身极低耗**：cpu 50m / mem 32Mi；重活在远端服务

### 9.2 并发策略

| 场景 | 策略 | 说明 |
|------|------|------|
| `up` 等待多组件 Ready | goroutine + WaitGroup | 并行探测各组件探针 |
| `model list` | 并行请求 | 同时打多个 model source endpoint |
| 端口转发 | goroutine 后台 | 启动后不阻塞主流程 |
| 日志跟踪 | goroutine + chan | 多 app 日志合并输出 |

```go
// up 并行等待 Ready
func waitAllReady(components []Component, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    var wg sync.WaitGroup
    errs := make(chan error, len(components))

    for _, c := range components {
        wg.Add(1)
        go func(comp Component) {
            defer wg.Done()
            if err := waitReady(ctx, comp); err != nil {
                errs <- fmt.Errorf("%s: %w", comp.Name, err)
            }
        }(c)
    }
    wg.Wait()
    close(errs)

    return firstOrNil(errs)
}
```

### 9.3 背压/取消

- 所有命令支持 `context.Context` + `Ctrl-C` 优雅取消
- `up` 就绪等待超时 30s（§13.4），超时后报错但保留部分状态供排查
- `--detach` 模式后台运行，不等待直接返回

### 9.4 性能规则

| # | 标题 | 触发条件 | 约束 | 示例 |
|---|------|----------|------|------|
| P1 | 并行探测 Ready | `up` 有多个组件 | goroutine + WaitGroup，30s 超时 | `go probe(comp, 30s)` |
| P2 | 并行打端点 | `model list` 多 source | 同时请求，取最快响应 | `go fetch(src, ch)` |
| P3 | Context 传播 | 所有命令 | 支持 `Ctrl-C` 取消 | `cmd.SetContext(ctx)` |
| P4 | 复用 HTTP 连接 | 远端调用 | http.Client 连接池 | `client.Timeout = 10s` |
| P5 | Profile 本地缓存 | 离线/慢网络 | 本地 ~/.openstrata/cache/ | `if !online: loadCache()` |
| P6 | 输出分页 | 长列表 | `model list` 默认 20 条/页 | `--limit 20 --offset 0` |

---

## §12 安全

### 12.1 安全边界

CLI 持平台 API Token，直接操作生产环境。安全隐患集中在：
- Token 泄漏（本地存储）
- 未授权的平台操作
- 输入注入（Manifest 写入）

### 12.2 安全规则

| # | 标题 | 触发条件 | 约束 | 示例 |
|---|------|----------|------|------|
| S1 | Token 加密存储 | auth login / config set token | 不落明文，本地 AES-GCM 加密 | `encrypt(token, machineID)` |
| S2 | Token 来源 | CLI 启动 | 优先级：env `OPENSTRATA_TOKEN` > `~/.openstrata/tokens/` > `--token` flag | `os.Getenv("OPENSTRATA_TOKEN")` |
| S3 | Schema 校验写入 | `config set` / `init` | 非法 key/val 拒绝写入 | `if !validKey(key): err("unknown key")` |
| S4 | Keycloak 鉴权 | 多用户场景 | 所有 API 调用附带 JWT（§4.7.3） | `Authorization: Bearer <jwt>` |
| S5 | 限流保护 | 反复调用网关 | 基础风控在网关侧约束 CLI 调用频率 | `429 Too Many Requests → retry-after` |
| S6 | 审计委托 | 所有变更操作 | CLI 本身不做审计，由服务端记录（§13.5） | `POST /v1/apply → 服务端写审计日志` |
| S7 | Manifest 路径校验 | `init`/`config` | 仅允许当前目录下写入，禁止 `../../` 穿越 | `filepath.Clean(path)` + 白名单 |
| S8 | 敏感信息脱敏 | `--verbose` 输出 | 不打印 Token/Secret 明文 | `maskSensitive(logEntry)` |

### 12.3 认证流程

```
用户 ──→ aictl login ──→ Keycloak OIDC
         │                     │
         │◄── JWT token ───────┘
         │
         └──→ 加密存储到 ~/.openstrata/tokens/

aictl up /
aictl apply ──→ Header: Authorization: Bearer <jwt>
                   │
                   ▼
              各服务端验证 JWT → 执行操作
```

### 12.4 风险场景与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Token 明文落盘 | 凭证泄漏 | AES-GCM 加密 + 文件权限 0600 |
| Manifest 注入非法配置 | 破坏运行态 | Schema 校验 + 值域约束 |
| 未授权 apply 操作 | 非法变更生产环境 | 服务端 JWT 鉴权 + RBAC |
| 批量命令 DoS | 打垮平台 API | 网关侧限流 + 命令间隔 |
| 离线环境 token 过期 | 无法操作 | 本地缓存 + refresh token 机制 |

---

> 命令树与退出码参见 [specs/SPECS.md](../specs/SPECS.md)
> 包结构与端口定义参见 [arch/ARCH.md](../arch/ARCH.md)
> 完整流程参见 [design/DESIGN.md §4](../design/DESIGN.md#4-处理流水线--请求路径)
