package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
)

const auditWriteTimeout = 2 * time.Second

type auditCreator interface {
	Create(ctx context.Context, log *model.AuditLog) error
}

type AuditEvent struct {
	UserID     string
	Action     string
	Resource   string
	ResourceID string
	ClientID   string
	IP         string
	UserAgent  string
	Source     string
	Status     string
	Detail     string
	Metadata   map[string]string
}

func writeAudit(ctx context.Context, audit auditCreator, entry *model.AuditLog) error {
	if audit == nil {
		logger.L.Warn().Str("action", entry.Action).Str("resource", entry.Resource).Msg("audit store is nil, skipping audit write")
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	auditCtx, cancel := context.WithTimeout(ctx, auditWriteTimeout)
	defer cancel()
	err := audit.Create(auditCtx, entry)
	if err != nil {
		RecordAuditDropped()
		logger.L.Warn().Err(err).Str("action", entry.Action).Str("resource", entry.Resource).Msg("failed to write audit log")
	}
	return err
}

func recordAuditEvent(ctx context.Context, audit auditCreator, event AuditEvent) error {
	return writeAudit(ctx, audit, &model.AuditLog{
		UserID:     event.UserID,
		Action:     event.Action,
		Resource:   event.Resource,
		ResourceID: event.ResourceID,
		ClientID:   event.ClientID,
		IP:         event.IP,
		UserAgent:  event.UserAgent,
		Detail:     formatAuditDetail(event.Source, event.Detail, event.Metadata),
		Status:     event.Status,
	})
}

func recordAuditFromFiber(c fiber.Ctx, audit auditCreator, event AuditEvent) error {
	ip, ua := GetRealIPAndUA(c)
	if event.IP == "" {
		event.IP = ip
	}
	if event.UserAgent == "" {
		event.UserAgent = ua
	}
	if event.Source == "" {
		if source, _ := c.Locals(auditSourceLocalKey).(string); source != "" {
			event.Source = source
		} else {
			event.Source = ResolveAuditSource(c)
		}
	}
	if requestID, _ := c.Locals("request_id").(string); requestID != "" {
		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}
		if _, ok := event.Metadata["request_id"]; !ok {
			event.Metadata["request_id"] = requestID
		}
	}
	return recordAuditEvent(c.Context(), audit, event)
}

func formatAuditDetail(source, detail string, metadata map[string]string) string {
	parts := make([]string, 0, len(metadata)+2)
	if source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", source))
	}
	if detail = strings.TrimSpace(detail); detail != "" {
		parts = append(parts, fmt.Sprintf("message=%s", detail))
	}
	if len(metadata) > 0 {
		keys := make([]string, 0, len(metadata))
		for key := range metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := strings.TrimSpace(metadata[key])
			if value == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return strings.Join(parts, " ")
}
