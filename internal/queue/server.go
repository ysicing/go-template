package queue

import (
	"context"

	"github.com/hibiken/asynq"
)

// ServerConfig 控制 worker 并发和队列权重。
type ServerConfig struct {
	Redis       RedisConfig
	Concurrency int
	Queues      map[string]int
}

// Server 包装 asynq worker server，并集中管理任务 handler 注册。
type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// NewServer 创建异步任务 worker server。
func NewServer(cfg ServerConfig) *Server {
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	queues := cfg.Queues
	if len(queues) == 0 {
		queues = map[string]int{"default": 1}
	}
	return &Server{
		server: asynq.NewServer(redisClientOpt(cfg.Redis), asynq.Config{
			Concurrency: concurrency,
			Queues:      queues,
		}),
		mux: asynq.NewServeMux(),
	}
}

// HandleFunc 注册指定任务类型的处理函数。
func (s *Server) HandleFunc(taskType string, handler func(context.Context, *asynq.Task) error) {
	if s == nil || s.mux == nil {
		return
	}
	s.mux.HandleFunc(taskType, handler)
}

// Start 启动 worker goroutine，但不接管进程信号。
func (s *Server) Start() error {
	if s == nil || s.server == nil || s.mux == nil {
		return ErrDisabled
	}
	return s.server.Start(s.mux)
}

// Shutdown 优雅停止 worker，等待正在执行的任务退出。
func (s *Server) Shutdown() {
	if s == nil || s.server == nil {
		return
	}
	s.server.Shutdown()
}
