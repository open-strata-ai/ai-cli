// Package adapter implements the OpenStrata platform anti-corrosion clients.
// PlatformClientImpl aggregates the resolver / provisioner / platform-api /
// eval-service control planes into a single domain.PlatformClient; GatewayClient
// is the OpenAI-compatible gateway client used for model management.
//
// An exported FakePlatformClient is provided so the CLI's use cases and command
// tests run fully offline (no live services required).
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/open-strata-ai/ai-cli/domain"
)

// PlatformClientImpl is the HTTP implementation of domain.PlatformClient.
type PlatformClientImpl struct {
	endpoint   string
	httpClient *http.Client
	gateway    *GatewayClient
}

// NewPlatformClient builds a client targeting the platform control plane.
func NewPlatformClient(endpoint string) *PlatformClientImpl {
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}
	return &PlatformClientImpl{
		endpoint:   strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		gateway:    NewGatewayClient(endpoint),
	}
}

func (c *PlatformClientImpl) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return domain.ErrGeneral("marshal request", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, rdr)
	if err != nil {
		return domain.ErrGeneral("build request", err)
	}
	req.Header.Set("content-type", "application/json")
	if resp, err := c.httpClient.Do(req); err != nil {
		return domain.ErrGeneral("request "+path, err)
	} else {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			if resp.StatusCode == 409 {
				return domain.ErrConflict(fmt.Sprintf("assembly conflict: %s", string(data)), nil)
			}
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return domain.ErrAuth(fmt.Sprintf("unauthorized: %s", string(data)), nil)
			}
			return domain.ErrGeneral(fmt.Sprintf("status %d: %s", resp.StatusCode, string(data)), nil)
		}
		if out != nil && len(data) > 0 {
			return json.Unmarshal(data, out)
		}
		return nil
	}
}

func (c *PlatformClientImpl) Init(ctx context.Context, profile, model string) error {
	return c.do(ctx, "POST", "/v1/init", map[string]string{"profile": profile, "model": model}, nil)
}

func (c *PlatformClientImpl) Up(ctx context.Context, profile string) error {
	return c.do(ctx, "POST", "/v1/up", map[string]string{"profile": profile}, nil)
}

func (c *PlatformClientImpl) Plan(ctx context.Context, enable []string, tenant string) (string, error) {
	var out struct {
		Checksum string `json:"checksum"`
	}
	err := c.do(ctx, "POST", "/v1/resolve", map[string]any{"enabled": enable, "tenant": tenant}, &out)
	return out.Checksum, err
}

func (c *PlatformClientImpl) Apply(ctx context.Context, checksum string) error {
	return c.do(ctx, "POST", "/v1/apply", map[string]string{"plan": checksum}, nil)
}

func (c *PlatformClientImpl) Rollback(ctx context.Context, component string) error {
	return c.do(ctx, "POST", "/v1/rollback", map[string]string{"component": component}, nil)
}

func (c *PlatformClientImpl) ListModels(ctx context.Context) ([]domain.ModelView, error) {
	return c.gateway.ListModels(ctx)
}

func (c *PlatformClientImpl) EnableModel(ctx context.Context, modelID string) error {
	return c.gateway.EnableModel(ctx, modelID)
}

func (c *PlatformClientImpl) DisableModel(ctx context.Context, modelID string) error {
	return c.gateway.DisableModel(ctx, modelID)
}

func (c *PlatformClientImpl) DeployApp(ctx context.Context, specPath string) error {
	return c.do(ctx, "POST", "/v1/apps", map[string]string{"spec": specPath}, nil)
}

func (c *PlatformClientImpl) AppLogs(ctx context.Context, appName string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/v1/apps/"+appName+"/logs", nil)
	if err != nil {
		return nil, domain.ErrGeneral("build logs request", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, domain.ErrGeneral("logs request", err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, domain.ErrGeneral(fmt.Sprintf("logs status %d", resp.StatusCode), nil)
	}
	return resp.Body, nil
}

func (c *PlatformClientImpl) RunEval(ctx context.Context, taskPath string) (string, error) {
	var out struct {
		TaskID string `json:"task_id"`
	}
	err := c.do(ctx, "POST", "/v1/eval", map[string]string{"spec": taskPath}, &out)
	return out.TaskID, err
}

func (c *PlatformClientImpl) EvalStatus(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	var out domain.EvalTaskResult
	err := c.do(ctx, "GET", "/v1/eval/"+taskID, nil, &out)
	return out, err
}

func (c *PlatformClientImpl) EvalResults(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	var out domain.EvalTaskResult
	err := c.do(ctx, "GET", "/v1/eval/"+taskID+"/results", nil, &out)
	return out, err
}

// GetConfig / SetConfig are handled locally via the manifest in the config
// use case; the platform port methods are no-ops here (SPECS §8.2: CLI holds no
// authoritative config data).
func (c *PlatformClientImpl) GetConfig(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (c *PlatformClientImpl) SetConfig(ctx context.Context, key, val string) error {
	return nil
}

func (c *PlatformClientImpl) PortForward(ctx context.Context, appName string, port int) error {
	return c.do(ctx, "POST", "/v1/port-forward", map[string]any{"app": appName, "port": port}, nil)
}

// ---------------------------------------------------------------------------
// FakePlatformClient — offline implementation for tests and local dry-runs.
// ---------------------------------------------------------------------------

// FakePlatformClient records calls and returns deterministic values.
type FakePlatformClient struct {
	Plans      []string
	Applied    []string
	RolledBack []string
	UpProfiles []string
	Inited     []string
	Deployed   []string
	Enabled    []string
	Disabled   []string
	Evals      []string
	PortFwd    []string
	Models     []domain.ModelView
}

// NewFakePlatformClient returns a FakePlatformClient seeded with two models.
func NewFakePlatformClient() *FakePlatformClient {
	return &FakePlatformClient{
		Models: []domain.ModelView{
			{ModelID: "qwen-cloud", Source: "aliyun", Enabled: true, Health: "healthy", Latency: "120ms"},
			{ModelID: "openai", Source: "openai", Enabled: false, Health: "unknown", Latency: "-"},
		},
	}
}

func (f *FakePlatformClient) Init(ctx context.Context, profile, model string) error {
	f.Inited = append(f.Inited, profile+":"+model)
	return nil
}
func (f *FakePlatformClient) Up(ctx context.Context, profile string) error {
	f.UpProfiles = append(f.UpProfiles, profile)
	return nil
}
func (f *FakePlatformClient) Plan(ctx context.Context, enable []string, tenant string) (string, error) {
	sum := tenant
	for _, e := range enable {
		sum += "," + e
	}
	cs := fmt.Sprintf("cs_%x", hashString(sum))
	f.Plans = append(f.Plans, cs)
	return cs, nil
}
func (f *FakePlatformClient) Apply(ctx context.Context, checksum string) error {
	f.Applied = append(f.Applied, checksum)
	return nil
}
func (f *FakePlatformClient) Rollback(ctx context.Context, component string) error {
	f.RolledBack = append(f.RolledBack, component)
	return nil
}
func (f *FakePlatformClient) ListModels(ctx context.Context) ([]domain.ModelView, error) {
	return f.Models, nil
}
func (f *FakePlatformClient) EnableModel(ctx context.Context, modelID string) error {
	f.Enabled = append(f.Enabled, modelID)
	return nil
}
func (f *FakePlatformClient) DisableModel(ctx context.Context, modelID string) error {
	f.Disabled = append(f.Disabled, modelID)
	return nil
}
func (f *FakePlatformClient) DeployApp(ctx context.Context, specPath string) error {
	f.Deployed = append(f.Deployed, specPath)
	return nil
}
func (f *FakePlatformClient) AppLogs(ctx context.Context, appName string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("fake logs for " + appName)), nil
}
func (f *FakePlatformClient) RunEval(ctx context.Context, taskPath string) (string, error) {
	id := fmt.Sprintf("eval_%x", hashString(taskPath))
	f.Evals = append(f.Evals, id)
	return id, nil
}
func (f *FakePlatformClient) EvalStatus(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	return domain.EvalTaskResult{TaskID: taskID, Status: "done", Score: 0.95, Duration: "3.2s"}, nil
}
func (f *FakePlatformClient) EvalResults(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	return domain.EvalTaskResult{TaskID: taskID, Status: "done", Score: 0.95, Duration: "3.2s"}, nil
}
func (f *FakePlatformClient) GetConfig(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (f *FakePlatformClient) SetConfig(ctx context.Context, key, val string) error {
	return nil
}
func (f *FakePlatformClient) PortForward(ctx context.Context, appName string, port int) error {
	f.PortFwd = append(f.PortFwd, fmt.Sprintf("%s:%d", appName, port))
	return nil
}

func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// Ensure both implement the port.
var (
	_ domain.PlatformClient = (*PlatformClientImpl)(nil)
	_ domain.PlatformClient = (*FakePlatformClient)(nil)
)
