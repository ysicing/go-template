package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
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

	before := readCounterMetricValue(t, "id_audit_dropped_total")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entry := &model.AuditLog{Action: model.AuditLogin, Resource: "user", Status: "failure"}
	_ = writeAudit(ctx, as, entry)

	after := readCounterMetricValue(t, "id_audit_dropped_total")
	if after != before+1 {
		t.Fatalf("expected dropped audit metric to increase by 1, before=%v after=%v", before, after)
	}
}

func readCounterMetricValue(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		if len(family.Metric) == 0 || family.Metric[0].Counter == nil {
			t.Fatalf("counter metric %q is empty", name)
		}
		return family.Metric[0].Counter.GetValue()
	}
	t.Fatalf("counter metric %q not found", name)
	return 0
}
