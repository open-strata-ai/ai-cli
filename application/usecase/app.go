package usecase

import (
	"context"

	"github.com/open-strata-ai/ai-cli/domain"
)

// AppUseCase deploys and debugs Agent applications via ai-platform-api (R5/R8).
type AppUseCase struct {
	client domain.PlatformClient
}

// NewAppUseCase constructs an AppUseCase.
func NewAppUseCase(client domain.PlatformClient) *AppUseCase {
	return &AppUseCase{client: client}
}

// Deploy deploys an Agent application from a spec file.
func (u *AppUseCase) Deploy(ctx context.Context, specPath string) error {
	if specPath == "" {
		return domain.ErrConfig("app deploy requires <spec.yaml>", nil)
	}
	return u.client.DeployApp(ctx, specPath)
}

// Logs streams logs for an application.
func (u *AppUseCase) Logs(ctx context.Context, appName string) (domain.ReadCloser, error) {
	if appName == "" {
		return nil, domain.ErrConfig("app logs requires <app>", nil)
	}
	return u.client.AppLogs(ctx, appName)
}

// PortForward opens a local port forward to an application.
func (u *AppUseCase) PortForward(ctx context.Context, appName string, port int) error {
	if appName == "" {
		return domain.ErrConfig("app port-forward requires <app>", nil)
	}
	return u.client.PortForward(ctx, appName, port)
}
