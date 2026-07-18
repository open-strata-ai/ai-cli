# ai-cli · Algorithms/Concurrency/Safety Rules

> Corresponding design documents §5 (key algorithms), §9 (concurrency and performance), §12 (observability/security)

---

## §5 Key algorithm

### 5.1 Guided initialization (`aictl init`)

**Input**: `--profile <p> --model <m>` + optional `--tenant <t>`
**Output**: `openstrata.yaml` (PlatformManifest)

```
function init(profile, model, tenant):
    skeleton = loadProfile(profile)        // openstrata-meta/profiles/<p>.yaml
    defaults = skeleton.defaults           // external, optional_disabled
    merged   = deepMerge(defaults, {
        profile: profile,
        model:   model,
        tenant:  tenant || "default",
        enabled: skeleton.enabled          //profile recommended enable list
    })

    validateSchema(merged)                  //Alignment §12.1 schema
    writeYAML("openstrata.yaml", merged)

    return manifestPath
```

**Schema validation rules**:
- `profile` must be one of the four profiles (starter/standard/advanced/full)
- `enabled` key must exist in Registry
- `model` is not empty (starter selects at least one LLM supplier)

### 5.2 One-click pull up (`aictl up`)

```
function up(profile, detach):
    renderer = selectRenderer(profile)
    // starter → Compose; standard+/advanced/full → Helm/K8s

    plan = callResolver.plan(profile)       //Call ai-dependency-resolver
    result = callProvisioner.apply(plan)     //Call ai-provisioning-engine

    if not detach:
        //Local development: port forwarding + detection Ready
        portForward(gateway, 8080:80)
        portForward(ui, 3000:80)

        err = waitForReady([
            {name: "gateway", url: "http://localhost:8080/healthz"},
            {name: "ui",      url: "http://localhost:3000"},
        ], timeout=30s)

        if err == nil:
            openBrowser("http://localhost:3000") // Chat UI
```

### 5.3 Assembly pass-through

`plan`/`apply`/`rollback` subcommands **zero business logic** - only collect parameters, call PlatformClient, and format output:

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

### 5.4 Configure reading and writing

```
function configSet(key, val):
    manifest = loadManifest("openstrata.yaml")
    manifest.set(key, val)
    validateSchema(manifest)          //Verify before writing
    writeYAML("openstrata.yaml", manifest)

function configGet(key):
    manifest = loadManifest("openstrata.yaml")
    return manifest.get(key)
```

### 5.5 Profile merge strategy

```
priority（high→Low）：
1. command line --enable flag（Explicitly specified）
2. openstrata.yaml The user has configured
3. profiles/<p>.yaml skeleton（external + optional_disabled）
4. global default（bom.yaml default value）
```

---

## §9 Concurrency and performance

### 9.1 Execution model

- **Framework**: Cobra single binary, no permanent service
- **Independent process for each command**: exit immediately after initiating (except `up --detach`)
- **CLI itself is extremely low consumption**: cpu 50m / mem 32Mi; re-activated in remote service

### 9.2 Concurrency strategy

| Scenario | Strategy | Description |
|------|------|------|
| `up` Wait for multiple components to be Ready | goroutine + WaitGroup | Detect each component probe in parallel |
| `model list` | Parallel requests | Make multiple model source endpoints at the same time |
| Port forwarding | goroutine background | Does not block the main process after startup |
| Log tracking | goroutine + chan | Multiple app log merge output |

```go
//up Parallel wait for Ready
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

### 9.3 Backpressure/Cancellation

- All commands support `context.Context` + `Ctrl-C` graceful cancellation
- `up` wait timeout is 30s (§13.4). After the timeout, an error will be reported but some status will be retained for troubleshooting.
- `--detach` mode runs in the background and returns directly without waiting.

### 9.4 Performance Rules

| # | Title | Trigger Condition | Constraints | Example |
|---|------|----------|------|------|
| P1 | Parallel probe Ready | `up` has multiple components | goroutine + WaitGroup, 30s timeout | `go probe(comp, 30s)` |
| P2 | Parallel endpoints | `model list` multiple sources | Simultaneous requests, get the fastest response | `go fetch(src, ch)` |
| P3 | Context propagation | All commands | Supports `Ctrl-C` to cancel | `cmd.SetContext(ctx)` |
| P4 | Reuse HTTP connection | Remote call | http.Client connection pool | `client.Timeout = 10s` |
| P5 | Profile local cache | Offline/slow network | Local ~/.openstrata/cache/ | `if !online: loadCache()` |
| P6 | Output pagination | Long list | `model list` default 20 items/page | `--limit 20 --offset 0` |

---

## §12 Security

### 12.1 Security Boundary

The CLI holds the platform API Token and directly operates the production environment. Security risks are concentrated in:
- Token leakage (local storage)
- Unauthorized platform operation
- Input injection (Manifest writing)

### 12.2 Security rules

| # | Title | Trigger Condition | Constraints | Example |
|---|------|----------|------|------|
| S1 | Token encrypted storage | auth login / config set token | No clear text, local AES-GCM encryption | `encrypt(token, machineID)` |
| S2 | Token source | CLI startup | Priority: env `OPENSTRATA_TOKEN` > `~/.openstrata/tokens/` > `--token` flag | `os.Getenv("OPENSTRATA_TOKEN")` |
| S3 | Schema verification writing | `config set` / `init` | Illegal key/val rejects writing | `if !validKey(key): err("unknown key")` |
| S4 | Keycloak Authentication | Multi-user scenarios | All API calls come with JWT (§4.7.3) | `Authorization: Bearer <jwt>` |
| S5 | Rate limiting protection | Repeated calls to the gateway | Basic risk control restricts the frequency of CLI calls on the gateway side | `429 Too Many Requests → retry-after` |
| S6 | Audit delegation | All change operations | CLI itself does not perform auditing, it is recorded by the server (§13.5) | `POST /v1/apply → The server writes audit logs` |
| S7 | Manifest path verification | `init`/`config` | Only allow writing in the current directory, prohibit `../../` traversal | `filepath.Clean(path)` + whitelist |
| S8 | Desensitization of sensitive information | `--verbose` output | Do not print Token/Secret plain text | `maskSensitive(logEntry)` |

### 12.3 Certification process

```
user ──→ aictl login ──→ Keycloak OIDC
         │                     │
         │◄── JWT token ───────┘
         │
         └──→ Encrypted storage to ~/.openstrata/tokens/

aictl up /
aictl apply ──→ Header: Authorization: Bearer <jwt>
                   │
                   ▼
              Verification of each server JWT → perform operations
```

### 12.4 Risk Scenarios and Mitigation

| Risk | Impact | Mitigation |
|------|------|------|
| Token plain text file | Credential leakage | AES-GCM encryption + file permissions 0600 |
| Manifest injects illegal configuration | Destroys running state | Schema verification + value range constraints |
| Unauthorized apply operation | Illegal changes to the production environment | Server-side JWT authentication + RBAC |
| Batch command DoS | Defeat the platform API | Gateway side flow limit + command interval |
| Offline environment token expired | Unable to operate | Local cache + refresh token mechanism |

---

> For the command tree and exit codes, see [docs/SPECS.md](./SPECS.md)
> For package structure and port definitions, see [docs/ARCH.md](./ARCH.md)
> For the complete process, see [docs/DESIGN.md §4](./DESIGN.md#4-Processing Pipeline--Request Path)
