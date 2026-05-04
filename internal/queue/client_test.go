package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type fakeEnqueueClient struct {
	task *asynq.Task
	opts []asynq.Option
	err  error
}

func (f *fakeEnqueueClient) EnqueueContext(_ context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.task = task
	f.opts = opts
	if f.err != nil {
		return nil, f.err
	}
	return &asynq.TaskInfo{ID: "task-1", Queue: "emails"}, nil
}

func TestClientEnqueueJSONBuildsAsynqTask(t *testing.T) {
	fake := &fakeEnqueueClient{}
	client := newClient(fake, nil)
	payload := map[string]string{"user_id": "u1", "email": "u1@example.com"}

	if err := client.Enqueue(context.Background(), Job{
		Type:     "email:verification",
		Payload:  payload,
		Queue:    "emails",
		MaxRetry: 3,
		Timeout:  30 * time.Second,
	}); err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	if fake.task == nil {
		t.Fatal("expected asynq task to be enqueued")
	}
	if fake.task.Type() != "email:verification" {
		t.Fatalf("expected task type email:verification, got %q", fake.task.Type())
	}
	var got map[string]string
	if err := json.Unmarshal(fake.task.Payload(), &got); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got["user_id"] != "u1" || got["email"] != "u1@example.com" {
		t.Fatalf("unexpected payload: %#v", got)
	}
	if len(fake.opts) != 3 {
		t.Fatalf("expected queue, max retry and timeout options, got %d", len(fake.opts))
	}
}

func TestClientEnqueueReturnsClientError(t *testing.T) {
	want := errors.New("redis unavailable")
	client := newClient(&fakeEnqueueClient{err: want}, nil)

	err := client.Enqueue(context.Background(), Job{Type: "email:verification", Payload: map[string]string{"user_id": "u1"}})
	if !errors.Is(err, want) {
		t.Fatalf("expected enqueue error %v, got %v", want, err)
	}
}

func TestDisabledClientReturnsErrDisabled(t *testing.T) {
	err := DisabledClient{}.Enqueue(context.Background(), Job{Type: "email:verification"})
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}
