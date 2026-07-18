package usecase

import (
	"context"

	"github.com/open-strata-ai/ai-cli/domain"
)

// ModelUseCase manages model suppliers via the gateway (DESIGN §2 R4).
type ModelUseCase struct {
	client domain.PlatformClient
}

// NewModelUseCase constructs a ModelUseCase.
func NewModelUseCase(client domain.PlatformClient) *ModelUseCase {
	return &ModelUseCase{client: client}
}

// List returns the configured model suppliers.
func (u *ModelUseCase) List(ctx context.Context) ([]domain.ModelView, error) {
	return u.client.ListModels(ctx)
}

// Enable enables a model supplier by ID.
func (u *ModelUseCase) Enable(ctx context.Context, modelID string) error {
	if modelID == "" {
		return domain.ErrConfig("model enable requires <model_id>", nil)
	}
	return u.client.EnableModel(ctx, modelID)
}

// Disable disables a model supplier by ID.
func (u *ModelUseCase) Disable(ctx context.Context, modelID string) error {
	if modelID == "" {
		return domain.ErrConfig("model disable requires <model_id>", nil)
	}
	return u.client.DisableModel(ctx, modelID)
}
