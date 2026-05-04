package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// ErrDisabled 表示当前运行时未启用异步队列。
var ErrDisabled = errors.New("queue disabled")

// Enqueuer 定义业务模块提交异步任务所需的最小接口。
type Enqueuer interface {
	Enqueue(ctx context.Context, job Job) error
}

// Job 描述一次待入队的 JSON 任务。
type Job struct {
	Type     string
	Payload  any
	Queue    string
	MaxRetry int
	Timeout  time.Duration
}

// RedisConfig 是 asynq 连接 Redis 所需的最小配置。
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type enqueueClient interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type closer interface {
	Close() error
}

// Client 封装 asynq.Client，统一 JSON payload 和任务选项构造。
type Client struct {
	enqueuer enqueueClient
	closer   closer
}

// NewClient 创建基于 asynq 的任务入队客户端。
func NewClient(redis RedisConfig) *Client {
	client := asynq.NewClient(redisClientOpt(redis))
	return newClient(client, client)
}

func newClient(enqueuer enqueueClient, closer closer) *Client {
	return &Client{enqueuer: enqueuer, closer: closer}
}

// Enqueue 将任务编码为 JSON 后提交给 asynq。
func (c *Client) Enqueue(ctx context.Context, job Job) error {
	if c == nil || c.enqueuer == nil {
		return ErrDisabled
	}
	task, opts, err := buildTask(job)
	if err != nil {
		return err
	}
	if _, err := c.enqueuer.EnqueueContext(ctx, task, opts...); err != nil {
		return err
	}
	return nil
}

// Close 释放底层 asynq client 连接。
func (c *Client) Close() error {
	if c == nil || c.closer == nil {
		return nil
	}
	return c.closer.Close()
}

// Ping 检查底层 Redis 连接是否可用。
func (c *Client) Ping() error {
	client, ok := c.enqueuer.(interface{ Ping() error })
	if !ok {
		return nil
	}
	return client.Ping()
}

// DisabledClient 用于队列未启用时保持调用方降级路径显式。
type DisabledClient struct{}

// Enqueue 对禁用队列固定返回 ErrDisabled。
func (DisabledClient) Enqueue(context.Context, Job) error {
	return ErrDisabled
}

func buildTask(job Job) (*asynq.Task, []asynq.Option, error) {
	if job.Type == "" {
		return nil, nil, fmt.Errorf("queue job type is required")
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal queue job payload: %w", err)
	}
	opts := make([]asynq.Option, 0, 3)
	if job.Queue != "" {
		opts = append(opts, asynq.Queue(job.Queue))
	}
	if job.MaxRetry > 0 {
		opts = append(opts, asynq.MaxRetry(job.MaxRetry))
	}
	if job.Timeout > 0 {
		opts = append(opts, asynq.Timeout(job.Timeout))
	}
	return asynq.NewTask(job.Type, payload), opts, nil
}

func redisClientOpt(redis RedisConfig) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     redis.Addr,
		Password: redis.Password,
		DB:       redis.DB,
	}
}
