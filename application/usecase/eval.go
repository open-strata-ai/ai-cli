package usecase

import (
	"context"

	"github.com/open-strata-ai/ai-cli/domain"
)

// EvalUseCase submits and queries evaluation tasks via ai-eval-service (R6).
type EvalUseCase struct {
	client domain.PlatformClient
}

// NewEvalUseCase constructs an EvalUseCase.
func NewEvalUseCase(client domain.PlatformClient) *EvalUseCase {
	return &EvalUseCase{client: client}
}

// Submit submits an evaluation task and returns its ID.
func (u *EvalUseCase) Submit(ctx context.Context, taskPath string) (string, error) {
	if taskPath == "" {
		return "", domain.ErrConfig("eval submit requires <task.yaml>", nil)
	}
	return u.client.RunEval(ctx, taskPath)
}

// Status returns the current status of an evaluation task.
func (u *EvalUseCase) Status(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	if taskID == "" {
		return domain.EvalTaskResult{}, domain.ErrConfig("eval status requires <id>", nil)
	}
	return u.client.EvalStatus(ctx, taskID)
}

// Results returns the results of a completed evaluation task.
func (u *EvalUseCase) Results(ctx context.Context, taskID string) (domain.EvalTaskResult, error) {
	if taskID == "" {
		return domain.EvalTaskResult{}, domain.ErrConfig("eval results requires <id>", nil)
	}
	return u.client.EvalResults(ctx, taskID)
}
