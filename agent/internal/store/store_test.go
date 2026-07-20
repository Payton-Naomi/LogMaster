package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Payton-Naomi/LogMaster/agent/internal/model"
)

func TestStoreRecoversClaimedBatchAndSequence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "agent.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(1); i <= 3; i++ {
		if err := s.Append(ctx, model.LogEntry{DeviceSN: "DUT1", Sequence: i, CapturedAt: time.Now().UTC(), Content: "line"}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ClaimBatch(ctx, "DUT1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Logs) != 2 {
		t.Fatalf("batch size=%d", len(first.Logs))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	recovered, err := s.ClaimBatch(ctx, "DUT1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.BatchID != first.BatchID {
		t.Fatalf("batch changed: %s != %s", recovered.BatchID, first.BatchID)
	}
	deleted, err := s.Acknowledge(ctx, recovered.BatchID)
	if err != nil || deleted != 2 {
		t.Fatalf("ack deleted=%d err=%v", deleted, err)
	}
	sequence, err := s.MaxSequence(ctx, "DUT1")
	if err != nil || sequence != 3 {
		t.Fatalf("sequence=%d err=%v", sequence, err)
	}
	remaining, err := s.PendingCount(ctx, "DUT1")
	if err != nil || remaining != 1 {
		t.Fatalf("remaining=%d err=%v", remaining, err)
	}
}
