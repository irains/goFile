package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/irains/fileharbor/auth"
	"github.com/irains/fileharbor/conf"
	"github.com/irains/fileharbor/utils"
)

const (
	testUploadID    = "upload-id-000001"
	testUploadIDTwo = "upload-id-000002"
	testUploadToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func newUploadStoreForTest(t *testing.T) (string, *RuntimeState, *UploadStore, auth.Info) {
	t.Helper()
	previousRoot := conf.FileHarbor
	root := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previousRoot })

	state, err := OpenRuntimeState(t.TempDir(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := state.Close(); err != nil {
			t.Error(err)
		}
	})
	config := defaultUploadConfig()
	config.MaxFileBytes = 2 << 20
	config.MaxPendingBytes = 4 << 20
	config.MinFreeBytes = 0
	store, err := NewUploadStore(state, config)
	if err != nil {
		t.Fatal(err)
	}
	return root, state, store, auth.Info{Username: "admin", SessionID: "stable-session"}
}

func TestUploadStoreCompletesIdempotentlyWithoutPersistingToken(t *testing.T) {
	root, _, store, info := newUploadStoreForTest(t)
	if err := os.Mkdir(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("reliable upload")
	digest := sha256Digest(contents)
	request := UploadCreateRequest{Path: "docs", Name: "report.txt", Size: int64(len(contents)), ExpectedSHA256: digest}

	status, created, err := store.Create(info, testUploadID, testUploadToken, request)
	if err != nil || !created || status.State != uploadStateActive || status.PartCount != 1 {
		t.Fatalf("create = %#v, created=%t, err=%v", status, created, err)
	}
	manifestPath := filepath.Join(store.uploadPath(testUploadID), uploadManifestFile)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestData), testUploadToken) {
		t.Fatal("upload manifest persisted the raw resume token")
	}

	if _, err := store.Status(info, testUploadID, strings.Repeat("f", 64)); !errors.Is(err, errUploadNotFound) {
		t.Fatalf("wrong capability status error = %v", err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, strings.Repeat("0", 64), bytes.NewReader(contents)); !errors.Is(err, errUploadInvalidDigest) {
		t.Fatalf("bad part digest error = %v", err)
	}

	status, repeated, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents))
	if err != nil || repeated || status.ReceivedBytes != int64(len(contents)) || len(status.Received) != 1 {
		t.Fatalf("first part = %#v, repeated=%t, err=%v", status, repeated, err)
	}
	_, repeated, err = store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents))
	if err != nil || !repeated {
		t.Fatalf("idempotent part = repeated=%t, err=%v", repeated, err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, sha256Digest([]byte("different")), bytes.NewReader([]byte("different"))); !errors.Is(err, errUploadPartConflict) {
		t.Fatalf("conflicting duplicate part error = %v", err)
	}

	status, repeated, err = store.Complete(info, testUploadID, testUploadToken)
	if err != nil || repeated || status.State != uploadStateCompleted || status.FinalPath != "docs/report.txt" || status.FinalSHA256 != digest {
		t.Fatalf("complete = %#v, repeated=%t, err=%v", status, repeated, err)
	}
	published, err := os.ReadFile(filepath.Join(root, "docs", "report.txt"))
	if err != nil || !bytes.Equal(published, contents) {
		t.Fatalf("published contents = %q, %v", published, err)
	}
	if _, err := os.Stat(filepath.Join(store.uploadPath(testUploadID), uploadPartsDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed parts directory remains: %v", err)
	}
	status, repeated, err = store.Complete(info, testUploadID, testUploadToken)
	if err != nil || !repeated || status.State != uploadStateCompleted {
		t.Fatalf("idempotent complete = %#v, repeated=%t, err=%v", status, repeated, err)
	}
}

func TestUploadStoreRejectsDestinationCollisionAndInvalidMetadata(t *testing.T) {
	root, _, store, info := newUploadStoreForTest(t)
	contents := []byte("existing")
	if err := os.WriteFile(filepath.Join(root, "taken.txt"), contents, 0644); err != nil {
		t.Fatal(err)
	}
	for _, request := range []UploadCreateRequest{
		{Path: "../outside", Name: "report.txt", Size: 0},
		{Path: "", Name: uploadStagePrefix + "reserved.partial", Size: 0},
	} {
		if _, _, err := store.Create(info, testUploadID, testUploadToken, request); !errors.Is(err, errUploadInvalid) {
			t.Fatalf("invalid request %#v error = %v", request, err)
		}
	}

	request := UploadCreateRequest{Path: "", Name: "taken.txt", Size: int64(len(contents))}
	if _, _, err := store.Create(info, testUploadIDTwo, testUploadToken, request); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadIDTwo, testUploadToken, 0, sha256Digest(contents), bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Complete(info, testUploadIDTwo, testUploadToken); utils.ErrorCode(err) != "destination_exists" {
		t.Fatalf("destination collision error = %v", err)
	}
}

func TestUploadStoreRecoversVerifiedPartsAndRejectsTampering(t *testing.T) {
	previousRoot := conf.FileHarbor
	root := t.TempDir()
	stateDir := t.TempDir()
	conf.FileHarbor = root
	t.Cleanup(func() { conf.FileHarbor = previousRoot })
	config := defaultUploadConfig()
	config.MaxFileBytes = 2 << 20
	config.MaxPendingBytes = 4 << 20
	config.MinFreeBytes = 0
	info := auth.Info{Username: "admin", SessionID: "stable-session"}
	contents := []byte("resume across restart")
	digest := sha256Digest(contents)

	state, err := OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewUploadStore(state, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "resumed.txt", Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}

	state, err = OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	store, err = NewUploadStore(state, config)
	if err != nil {
		t.Fatalf("restart recovery rejected verified state: %v", err)
	}
	defer state.Close()
	status, err := store.Status(info, testUploadID, testUploadToken)
	if err != nil || status.ReceivedBytes != int64(len(contents)) {
		t.Fatalf("resumed status = %#v, %v", status, err)
	}

	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	partPath, err := store.partPath(&UploadManifest{ID: testUploadID, PartCount: 1}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partPath, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	state, err = OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	if _, err := NewUploadStore(state, config); err == nil {
		t.Fatal("restart recovery accepted a tampered part")
	}
}

func TestUploadStoreRetriesFinalizingUploadWithoutRestart(t *testing.T) {
	root, _, store, info := newUploadStoreForTest(t)
	contents := []byte("retry finalizing upload")
	digest := sha256Digest(contents)
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "retry.txt", Size: int64(len(contents)), ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}

	manifest, err := store.loadManifest(testUploadID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.State = uploadStateFinalizing
	manifest.StageName = uploadStageName(testUploadID)
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	stage, err := store.stagePath(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte("stale incomplete stage"), 0600); err != nil {
		t.Fatal(err)
	}

	status, repeated, err := store.Complete(info, testUploadID, testUploadToken)
	if err != nil || repeated || status.State != uploadStateCompleted || status.FinalPath != "retry.txt" || status.FinalSHA256 != digest {
		t.Fatalf("retry complete = %#v, repeated=%t, err=%v", status, repeated, err)
	}
	published, err := os.ReadFile(filepath.Join(root, "retry.txt"))
	if err != nil || !bytes.Equal(published, contents) {
		t.Fatalf("published retry contents = %q, %v", published, err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staging file remains after retry: %v", err)
	}
}

func TestUploadStoreCompletesPublishedFinalizationOnRetryWithoutRestart(t *testing.T) {
	root, _, store, info := newUploadStoreForTest(t)
	contents := []byte("complete published finalization")
	digest := sha256Digest(contents)
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "published.txt", Size: int64(len(contents)), ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.loadManifest(testUploadID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.State = uploadStateFinalizing
	manifest.StageName = uploadStageName(testUploadID)
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "published.txt"), contents, 0644); err != nil {
		t.Fatal(err)
	}

	status, repeated, err := store.Complete(info, testUploadID, testUploadToken)
	if err != nil || !repeated || status.State != uploadStateCompleted || status.FinalPath != "published.txt" || status.FinalSHA256 != digest {
		t.Fatalf("published retry complete = %#v, repeated=%t, err=%v", status, repeated, err)
	}
	manifest, err = store.loadManifest(testUploadID)
	if err != nil || manifest.State != uploadStateCompleted || len(manifest.Parts) != 0 {
		t.Fatalf("completed manifest = %#v, err=%v", manifest, err)
	}
}

func TestUploadStoreDoesNotCancelPublishedFinalization(t *testing.T) {
	root, _, store, info := newUploadStoreForTest(t)
	contents := []byte("published finalization cannot be cancelled")
	digest := sha256Digest(contents)
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "cannot-cancel.txt", Size: int64(len(contents)), ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.loadManifest(testUploadID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.State = uploadStateFinalizing
	manifest.StageName = uploadStageName(testUploadID)
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cannot-cancel.txt"), contents, 0644); err != nil {
		t.Fatal(err)
	}

	if err := store.Cancel(info, testUploadID, testUploadToken); !errors.Is(err, errUploadConflict) {
		t.Fatalf("cancel published finalization error = %v", err)
	}
	status, err := store.Status(info, testUploadID, testUploadToken)
	if err != nil || status.State != uploadStateCompleted || status.FinalPath != "cannot-cancel.txt" {
		t.Fatalf("status after rejected cancel = %#v, err=%v", status, err)
	}
}

func TestUploadStoreRecoversPublishedDestinationBeforeCompletedManifest(t *testing.T) {
	root, state, store, info := newUploadStoreForTest(t)
	contents := []byte("recover published destination")
	digest := sha256Digest(contents)
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "recovered.txt", Size: int64(len(contents)), ExpectedSHA256: digest}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "recovered.txt"), contents, 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.loadManifest(testUploadID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.State = uploadStateFinalizing
	manifest.StageName = uploadStageName(testUploadID)
	manifest.FinalSHA256 = ""
	if err := store.saveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	stateDir := state.Dir
	config := store.config
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}

	state, err = OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	store, err = NewUploadStore(state, config)
	if err != nil {
		t.Fatalf("publication recovery failed: %v", err)
	}
	defer state.Close()
	status, err := store.Status(info, testUploadID, testUploadToken)
	if err != nil || status.State != uploadStateCompleted || status.FinalSHA256 != digest || status.FinalPath != "recovered.txt" {
		t.Fatalf("recovered status = %#v, err=%v", status, err)
	}
}

func TestUploadStoreRestartCleansCompletedParts(t *testing.T) {
	root, state, store, info := newUploadStoreForTest(t)
	contents := []byte("completed cleanup")
	digest := sha256Digest(contents)
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "cleanup.txt", Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, digest, bytes.NewReader(contents)); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.loadManifest(testUploadID)
	if err != nil {
		t.Fatal(err)
	}
	manifest.State = uploadStateCompleted
	manifest.FinalPath = "cleanup.txt"
	manifest.FinalSHA256 = digest
	manifest.Parts = map[int]UploadPart{}
	manifest.CompletedAt = time.Now().UTC()
	manifest.UpdatedAt = manifest.CompletedAt
	manifest.ExpiresAt = manifest.CompletedAt.Add(store.config.CompletionTTL)
	if err := store.saveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cleanup.txt"), contents, 0644); err != nil {
		t.Fatal(err)
	}
	stateDir, config := state.Dir, store.config
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	state, err = OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	if _, err := NewUploadStore(state, config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state.UploadsDir, testUploadID, uploadPartsDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed parts directory remains after restart: %v", err)
	}
}

func TestUploadStoreReaperDoesNotRemoveActivePartWrite(t *testing.T) {
	_, _, store, info := newUploadStoreForTest(t)
	store.config.InactivityTTL = time.Minute
	start := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return start }
	contents := []byte("active write")
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "active.txt", Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		_, _, err := store.WritePart(info, testUploadID, testUploadToken, 0, sha256Digest(contents), &blockingUploadReader{Reader: bytes.NewReader(contents), started: started, release: release})
		writeDone <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("part write did not begin")
	}

	reapDone := make(chan error, 1)
	go func() { reapDone <- store.ReapExpired() }()
	select {
	case err := <-reapDone:
		t.Fatalf("reaper returned before active write ended: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-reapDone; err != nil {
		t.Fatal(err)
	}
	if _, err := store.Status(info, testUploadID, testUploadToken); err != nil {
		t.Fatalf("active transfer was reaped after its part refreshed expiry: %v", err)
	}
}

type blockingUploadReader struct {
	io.Reader
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (reader *blockingUploadReader) Read(buffer []byte) (int, error) {
	reader.once.Do(func() {
		close(reader.started)
		<-reader.release
	})
	return reader.Reader.Read(buffer)
}

func TestUploadStoreReapsExpiredTransfersWithPerTransferLock(t *testing.T) {
	_, _, store, info := newUploadStoreForTest(t)
	start := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return start }
	if _, _, err := store.Create(info, testUploadID, testUploadToken, UploadCreateRequest{Path: "", Name: "expired.txt", Size: 0}); err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return start.Add(store.config.InactivityTTL) }
	if err := store.ReapExpired(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Status(info, testUploadID, testUploadToken); !errors.Is(err, errUploadNotFound) {
		t.Fatalf("expired transfer status error = %v", err)
	}
}

func sha256Digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
