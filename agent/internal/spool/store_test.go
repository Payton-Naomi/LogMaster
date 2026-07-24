package spool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testFile(t *testing.T, dir, name, content string, sequence int64) File {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return File{Path: path, SHA256: hex.EncodeToString(sum[:]), SizeBytes: int64(len(content)), DeviceSN: "DVR-1", FirstSequence: sequence, LastSequence: sequence}
}

func TestUploadStateLifecycleAndUncertainRequiresOperator(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	file := testFile(t, t.TempDir(), "DVR-1_COM3_session_1-1.log", "ERROR one\n", 1)
	id, err := store.EnqueueFile(ctx, "DR2860", "V1", file)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || batch.ID != id || batch.State != Uploading {
		t.Fatalf("claim: batch=%+v err=%v", batch, err)
	}
	if err := store.MarkUncertain(ctx, id, "response lost"); err != nil {
		t.Fatal(err)
	}
	if next, err := store.ClaimReady(ctx, 10); err != nil || next != nil {
		t.Fatalf("uncertain batch was automatically retried: %+v %v", next, err)
	}
	if err := store.RetryUncertain(ctx, id); err != nil {
		t.Fatal(err)
	}
	batch, err = store.ClaimReady(ctx, 10)
	if err != nil || batch == nil {
		t.Fatalf("operator retry was not claimable: %v", err)
	}
	if err := store.MarkUploaded(ctx, id, "upload-id", "task-id"); err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetBatch(ctx, id)
	if err != nil || stored.State != Uploaded || stored.UploadID != "upload-id" || stored.TaskID != "task-id" {
		t.Fatalf("unexpected stored acknowledgement: %+v %v", stored, err)
	}
}

func TestClaimMergesFilesAndSplitRestoresIndividualPendingBatches(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	for i, name := range []string{"one.log", "two.log"} {
		if _, err := store.EnqueueFile(ctx, "project", "version", testFile(t, dir, name, name, int64(i+1))); err != nil {
			t.Fatal(err)
		}
	}
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || len(batch.Files) != 2 {
		t.Fatalf("expected merged batch: %+v %v", batch, err)
	}
	if err := store.SplitUploading(ctx, batch.ID); err != nil {
		t.Fatal(err)
	}
	first, _ := store.ClaimReady(ctx, 10)
	if first == nil || len(first.Files) != 1 {
		t.Fatalf("split files must not be merged into the rejected batch again: %+v", first)
	}
}

func TestAnalysisCacheExpires(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	if err := store.Put(ctx, "key", []byte(`{"summary":"ok"}`), time.Hour); err != nil {
		t.Fatal(err)
	}
	if value, ok, err := store.Get(ctx, "key"); err != nil || !ok || string(value) != `{"summary":"ok"}` {
		t.Fatalf("unexpected cache value: %s %v %v", value, ok, err)
	}
	store.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, ok, err := store.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("expired cache returned: %v %v", ok, err)
	}
}

func TestEnqueueFileIsIdempotentByPath(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	file := testFile(t, t.TempDir(), "one.log", "one", 1)
	first, err := store.EnqueueFile(ctx, "p", "v", file)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.EnqueueFile(ctx, "p", "v", file)
	if err != nil || second != first {
		t.Fatalf("ids = %q, %q; err=%v", first, second, err)
	}
	counts, _ := store.Counts(ctx)
	if counts[Pending] != 1 {
		t.Fatalf("pending count = %d", counts[Pending])
	}
}

func TestClaimReadyDoesNotMergeDevicesAndRecoverNeverLeavesUploading(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	first := testFile(t, dir, "one.log", "one", 1)
	second := testFile(t, dir, "two.log", "two", 1)
	second.DeviceSN = "DVR-2"
	if _, err := store.EnqueueFile(ctx, "p", "v", first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnqueueFile(ctx, "p", "v", second); err != nil {
		t.Fatal(err)
	}
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || len(batch.Files) != 1 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	store.now = func() time.Time { return batch.CreatedAt.Add(time.Second) }
	recovered, err := store.Recover(ctx, 24*time.Hour)
	if err != nil || recovered != 1 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
	stored, err := store.GetBatch(ctx, batch.ID)
	if err != nil || stored.State != Uncertain {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}
