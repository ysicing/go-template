package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

func TestWriteAuditReturnsNilWhenStoreIsNil(t *testing.T) {
	entry := &model.AuditLog{Action: model.AuditLogin, Resource: "user", Status: "success"}
	if err := writeAudit(context.Background(), nil, entry); err != nil {
		t.Fatalf("expected nil error when audit store is nil, got %v", err)
	}
}

func TestWriteAuditWithTimeoutWhenContextExpired(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	as := store.NewAuditLogStore(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entry := &model.AuditLog{Action: model.AuditLogin, Resource: "user", Status: "success"}
	err = writeAudit(ctx, as, entry)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context cancellation/deadline error, got %v", err)
	}
}

func TestWriteAuditStoresRecordOnSuccess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	as := store.NewAuditLogStore(db)

	entry := &model.AuditLog{
		UserID:     "u1",
		Action:     model.AuditLogin,
		Resource:   "user",
		ResourceID: "u1",
		Status:     "success",
		IP:         "127.0.0.1",
		UserAgent:  "test",
	}
	if err := writeAudit(context.Background(), as, entry); err != nil {
		t.Fatalf("expected writeAudit success, got %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var logs []model.AuditLog
	if err := db.WithContext(ctx).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(logs))
	}
}

func TestWriteAuditRecordsDroppedMetricOnFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	as := store.NewAuditLogStore(db)

	before := readCounterValue(t, auditDropped)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entry := &model.AuditLog{Action: model.AuditLogin, Resource: "user", Status: "failure"}
	_ = writeAudit(ctx, as, entry)

	after := readCounterValue(t, auditDropped)
	if after != before+1 {
		t.Fatalf("expected dropped audit metric to increase by 1, before=%v after=%v", before, after)
	}
}

func readCounterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	metric := &io_prometheus_client.Metric{}
	if err := counter.Write(metric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if metric.Counter == nil {
		t.Fatal("counter metric is nil")
	}
	return metric.Counter.GetValue()
}
