package emailhandler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/pkg/metrics"

	"github.com/hibiken/asynq"
)

// TypeVerificationEmailTask 是邮件验证发送任务的 asynq 类型名。
const TypeVerificationEmailTask = "email:verification"

type verificationEmailPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Body   string `json:"body"`
}

// HandleVerificationEmailTask 处理 asynq 邮件验证发送任务。
func (h *EmailHandler) HandleVerificationEmailTask(ctx context.Context, task *asynq.Task) error {
	var payload verificationEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode verification email task: %v: %w", err, asynq.SkipRetry)
	}
	return h.sendVerificationEmailPayload(ctx, payload)
}

func (h *EmailHandler) sendVerificationEmailPayload(ctx context.Context, payload verificationEmailPayload) error {
	if err := h.sendEmailWithContext(ctx, payload.Email, "Verify your email address", payload.Body); err != nil {
		logger.L.Warn().
			Err(err).
			Str("user_id", payload.UserID).
			Str("email", payload.Email).
			Msg("failed to send queued verification email")
		metrics.RecordEmailSent("verification", "failure")
		return err
	}
	logger.L.Info().
		Str("user_id", payload.UserID).
		Str("email", payload.Email).
		Msg("queued verification email sent successfully")
	metrics.RecordEmailSent("verification", "success")
	return nil
}
