package spool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestBatchUploadPositionFollowsCollectionSessionOrder(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	dir := t.TempDir()
	firstFile := testFile(t, dir, "first.log", "first\n", 1)
	firstFile.SessionID = "session-one"
	firstID, err := store.EnqueueFile(ctx, "project", "version", firstFile)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	secondFile := testFile(t, dir, "second.log", "second\n", 2)
	secondFile.SessionID = "session-one"
	secondID, err := store.EnqueueFile(ctx, "project", "version", secondFile)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.GetBatch(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.GetBatch(ctx, secondID)
	if err != nil {
		t.Fatal(err)
	}
	if first.UploadPosition != 1 || second.UploadPosition != 2 {
		t.Fatalf("unexpected upload positions: first=%d second=%d", first.UploadPosition, second.UploadPosition)
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

func TestDeleteLocalHistoryRecordRejectsQueuedFiles(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	file := testFile(t, dir, "history.log", "local history", 1)
	session := Session{ID: "session-delete", DeviceSN: file.DeviceSN, PortName: "COM3", SaveEnabled: true, StartedAt: time.Now()}
	logFileID, err := store.RegisterLogFile(ctx, session, LogFileRecord{Path: file.Path, DeviceSN: file.DeviceSN, PortName: "COM3", FirstSequence: 1, LastSequence: 1, SizeBytes: file.SizeBytes, SHA256: file.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteLocalHistoryRecord(ctx, logFileID); err != nil {
		t.Fatalf("delete local history record: %v", err)
	}
	if _, err := store.GetLogFile(ctx, logFileID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted history record still exists: %v", err)
	}

	queuedFile := testFile(t, dir, "queued.log", "queued history", 2)
	queuedID, err := store.RegisterLogFile(ctx, session, LogFileRecord{Path: queuedFile.Path, DeviceSN: queuedFile.DeviceSN, PortName: "COM3", FirstSequence: 2, LastSequence: 2, SizeBytes: queuedFile.SizeBytes, SHA256: queuedFile.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnqueueHistoryFile(ctx, queuedID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteLocalHistoryRecord(ctx, queuedID); err == nil {
		t.Fatal("queued history file must not be deleted")
	}
}

func TestClaimReadyDoesNotMergeDevices(t *testing.T) {
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
}

func TestRecoverMarksRecentUploadingPending(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	file := testFile(t, dir, "one.log", "one", 1)
	if _, err := store.EnqueueFile(ctx, "p", "v", file); err != nil {
		t.Fatal(err)
	}
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || len(batch.Files) != 1 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	recovered, err := store.Recover(ctx, 24*time.Hour)
	if err != nil || recovered != 0 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
	stored, err := store.GetBatch(ctx, batch.ID)
	if err != nil || stored.State != Pending {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}

func TestRecoverMarksStaleUploadingUncertain(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	file := testFile(t, dir, "one.log", "one", 1)
	if _, err := store.EnqueueFile(ctx, "p", "v", file); err != nil {
		t.Fatal(err)
	}
	claimed := time.Now().UTC()
	store.now = func() time.Time { return claimed }
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || len(batch.Files) != 1 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	store.now = func() time.Time { return claimed.Add(25 * time.Hour) }
	recovered, err := store.Recover(ctx, 24*time.Hour)
	if err != nil || recovered != 1 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
	stored, err := store.GetBatch(ctx, batch.ID)
	if err != nil || stored.State != Uncertain {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}

func TestPausedDeviceDoesNotBlockOtherUploadsAndResumesOriginalBatch(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	pausedFile := testFile(t, dir, "paused.log", "paused", 1)
	pausedFile.DeviceSN = "DVR-PAUSED"
	activeFile := testFile(t, dir, "active.log", "active", 1)
	activeFile.DeviceSN = "DVR-ACTIVE"
	pausedID, err := store.EnqueueFile(ctx, "p", "v", pausedFile)
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := store.EnqueueFile(ctx, "p", "v", activeFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetDeviceUploadPaused(ctx, pausedFile.DeviceSN, true); err != nil {
		t.Fatal(err)
	}
	batch, err := store.ClaimReady(ctx, 10)
	if err != nil || batch == nil || batch.ID != activeID {
		t.Fatalf("active device was blocked: batch=%+v err=%v", batch, err)
	}
	if err := store.MarkUploaded(ctx, activeID, "upload-active", "task-active"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDeviceUploadPaused(ctx, pausedFile.DeviceSN, false); err != nil {
		t.Fatal(err)
	}
	batch, err = store.ClaimReady(ctx, 10)
	if err != nil || batch == nil || batch.ID != pausedID || len(batch.Files) != 1 {
		t.Fatalf("paused batch did not resume: batch=%+v err=%v", batch, err)
	}
}

func TestUploadMetadataRoundTripsWithStableClientRequestID(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Date(2026, 8, 3, 1, 2, 3, 0, time.UTC)
	file := testFile(t, t.TempDir(), "metadata.log", "metadata", 1)
	file.SessionID = "session-1"
	metadata := UploadMetadata{ProjectID: "42", ProjectName: "Project A", Version: "V1", TestTaskID: "task-a", TestTaskName: "高温测试", UploaderName: "张三", Remark: "回归", CollectorVersion: "0.0.3", Timezone: "Asia/Shanghai", CreatedAt: &at, StartedAt: &at, ScenarioIDs: []string{"scene-a", "scene-b"}}
	id, err := store.EnqueueFileWithMetadata(ctx, metadata, file)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := store.GetBatch(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if batch.ClientRequestID != id || batch.ProjectID != metadata.ProjectID || batch.UploaderName != metadata.UploaderName || batch.Remark != metadata.Remark || batch.SourceStartedAt == nil || !batch.SourceStartedAt.Equal(at) || !reflect.DeepEqual(batch.ScenarioIDs, metadata.ScenarioIDs) {
		t.Fatalf("metadata did not round trip: %+v", batch)
	}
}

func TestRetryKeepsOriginalRequestBodyFrozen(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dir := t.TempDir()
	firstFile := testFile(t, dir, "first.log", "first", 1)
	firstFile.SessionID = "session-1"
	firstID, err := store.EnqueueFileWithMetadata(ctx, UploadMetadata{ProjectName: "p", Version: "v"}, firstFile)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.ClaimReady(ctx, 10)
	if err != nil || first.ID != firstID {
		t.Fatalf("first claim: %+v %v", first, err)
	}
	if err := store.MarkPending(ctx, first.ID, "retry", time.Time{}); err != nil {
		t.Fatal(err)
	}
	secondFile := testFile(t, dir, "second.log", "second", 2)
	secondFile.SessionID = "session-1"
	if _, err := store.EnqueueFileWithMetadata(ctx, UploadMetadata{ProjectName: "p", Version: "v"}, secondFile); err != nil {
		t.Fatal(err)
	}
	retry, err := store.ClaimReady(ctx, 10)
	if err != nil || retry == nil || retry.ID != firstID || len(retry.Files) != 1 {
		t.Fatalf("retry body changed: %+v %v", retry, err)
	}
}
