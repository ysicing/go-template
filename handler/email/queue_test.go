package emailhandler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ysicing/go-template/internal/queue"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/hibiken/asynq"
)

type fakeEmailQueue struct {
	jobs []queue.Job
	err  error
}

func (f *fakeEmailQueue) Enqueue(_ context.Context, job queue.Job) error {
	f.jobs = append(f.jobs, job)
	return f.err
}

func TestSendVerificationEmailEnqueuesVerificationTask(t *testing.T) {
	h, users, _, cache := setupEmailHandler(t)
	user := createTestUser(t, users, "queued@example.com", false)
	q := &fakeEmailQueue{}
	h.SetQueue(q)

	app := fiber.New()
	app.Get("/send", func(c fiber.Ctx) error {
		return h.SendVerificationEmail(c, user, "https://id.example.com")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/send", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(q.jobs) != 1 {
		t.Fatalf("expected one queued job, got %d", len(q.jobs))
	}
	job := q.jobs[0]
	if job.Type != TypeVerificationEmailTask {
		t.Fatalf("expected task type %q, got %q", TypeVerificationEmailTask, job.Type)
	}
	if job.Queue != "emails" {
		t.Fatalf("expected emails queue, got %q", job.Queue)
	}
	payload, ok := job.Payload.(verificationEmailPayload)
	if !ok {
		t.Fatalf("expected verificationEmailPayload, got %T", job.Payload)
	}
	if payload.UserID != user.ID || payload.Email != user.Email {
		t.Fatalf("unexpected payload user: %#v", payload)
	}
	if !strings.Contains(payload.Body, "https://id.example.com/verify-email?token=") {
		t.Fatalf("expected verification link in body, got %q", payload.Body)
	}

	ephemeral := store.NewEphemeralTokenStore(cache)
	token := strings.TrimPrefix(strings.Split(strings.Split(payload.Body, "verify-email?token=")[1], "<")[0], "")
	token = strings.Split(token, "\"")[0]
	if got, err := ephemeral.LoadString(context.Background(), "verify", "email", token); err != nil || got != user.ID {
		t.Fatalf("expected queued token to map to user, got user=%q err=%v", got, err)
	}
}

func TestHandleVerificationEmailTaskRejectsInvalidPayload(t *testing.T) {
	h, _, _, _ := setupEmailHandler(t)

	err := h.HandleVerificationEmailTask(context.Background(), asynq.NewTask(TypeVerificationEmailTask, []byte("{")))
	if err == nil {
		t.Fatal("expected invalid payload error")
	}
	if !strings.Contains(err.Error(), "decode verification email task") {
		t.Fatalf("unexpected error: %v", err)
	}
}
