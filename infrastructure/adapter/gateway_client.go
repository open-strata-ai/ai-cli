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

// GatewayClient is the OpenAI-compatible client for ai-gateway-core, used for
// model supplier management (DESIGN §3 / ARCH §6).
type GatewayClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewGatewayClient builds a gateway client targeting the gateway base URL.
func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetAPIKey attaches the bearer token used for authenticated calls.
func (g *GatewayClient) SetAPIKey(key string) { g.apiKey = key }

func (g *GatewayClient) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, rdr)
	if err != nil {
		return domain.ErrGeneral("build gateway request", err)
	}
	req.Header.Set("content-type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("authorization", "Bearer "+g.apiKey)
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return domain.ErrGeneral("gateway request", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return domain.ErrGeneral(fmt.Sprintf("gateway status %d: %s", resp.StatusCode, string(data)), nil)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// ListModels returns the configured model suppliers.
func (g *GatewayClient) ListModels(ctx context.Context) ([]domain.ModelView, error) {
	var out []domain.ModelView
	err := g.do(ctx, "GET", "/v1/models", nil, &out)
	return out, err
}

// EnableModel enables a model supplier.
func (g *GatewayClient) EnableModel(ctx context.Context, modelID string) error {
	return g.do(ctx, "POST", "/v1/models/"+modelID+"/enable", nil, nil)
}

// DisableModel disables a model supplier.
func (g *GatewayClient) DisableModel(ctx context.Context, modelID string) error {
	return g.do(ctx, "POST", "/v1/models/"+modelID+"/disable", nil, nil)
}
