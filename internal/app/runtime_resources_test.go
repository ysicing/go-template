package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ysicing/go-template/store"

	"github.com/rs/zerolog"
)

type stubCache struct {
	closed bool
}

type stubTaskClient struct {
	closed bool
}

func (s *stubTaskClient) Close() error {
	s.closed = true
	return nil
}

type stubTaskServer struct {
	started  bool
	shutdown bool
}

func (s *stubTaskServer) Start() error {
	s.started = true
	return nil
}

func (s *stubTaskServer) Shutdown() {
	s.shutdown = true
}

func (s *stubCache) Ping(_ context.Context) error { return nil }
func (s *stubCache) Close() error                 { s.closed = true; return nil }
func (s *stubCache) Get(_ context.Context, _ string) (string, error) {
	return "", errors.New("not implemented")
}
func (s *stubCache) Set(_ context.Context, _, _ string, _ time.Duration) error {
	return errors.New("not implemented")
}
func (s *stubCache) Del(_ context.Context, _ string) error { return errors.New("not implemented") }
func (s *stubCache) SetNX(_ context.Context, _, _ string, _ time.Duration) (bool, error) {
	return false, errors.New("not implemented")
}
func (s *stubCache) RefreshIfValue(_ context.Context, _, _ string, _ time.Duration) (bool, error) {
	return false, errors.New("not implemented")
}
func (s *stubCache) DelIfValue(_ context.Context, _, _ string) (bool, error) {
	return false, errors.New("not implemented")
}
func (s *stubCache) Incr(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return 0, errors.New("not implemented")
}
func (s *stubCache) GetInt(_ context.Context, _ string) (int64, error) {
	return 0, errors.New("not implemented")
}

func TestRuntimeResourcesClose_ClosesSessionStorageAndCache(t *testing.T) {
	cache := &stubCache{}
	taskClient := &stubTaskClient{}
	taskServer := &stubTaskServer{}
	sessionClosed := false
	resources := runtimeResources{
		cache:      cache,
		taskClient: taskClient,
		taskServer: taskServer,
		sessionStorage: store.SessionStorageResource{
			CloseFunc: func() error {
				sessionClosed = true
				return nil
			},
		},
	}
	log := zerolog.Nop()

	resources.close(&log)

	if !sessionClosed {
		t.Fatal("expected session storage to be closed")
	}
	if !cache.closed {
		t.Fatal("expected cache to be closed")
	}
	if !taskClient.closed {
		t.Fatal("expected task queue client to be closed")
	}
	if !taskServer.shutdown {
		t.Fatal("expected task queue server to be shutdown")
	}
}

var _ store.Cache = (*stubCache)(nil)
