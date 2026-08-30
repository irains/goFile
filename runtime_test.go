package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpenRuntimeStateCreatesPrivateLayoutAndClearsStaleData(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	for _, path := range []string{
		filepath.Join(stateDir, stateChunksDirectory, "stale"),
		filepath.Join(stateDir, stateTempDirectory, "stale.zip"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stale"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state, err := OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	if state.Ready() {
		t.Fatal("state should not become ready before the server starts")
	}
	for _, path := range []string{state.ChunksDir, state.TempDir, filepath.Join(state.Dir, stateAuditFile)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required state path %q: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(state.ChunksDir, "stale"),
		filepath.Join(state.TempDir, "stale.zip"),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale state path %q was not cleared: %v", path, err)
		}
	}
}

func TestOpenRuntimeStateRejectsOverlapAndStateSymlink(t *testing.T) {
	root := t.TempDir()
	if _, err := OpenRuntimeState(root, root); err == nil {
		t.Fatal("state directory overlapping managed root was accepted")
	}
	stateParent := t.TempDir()
	managed := filepath.Join(stateParent, "managed")
	if err := os.Mkdir(managed, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRuntimeState(stateParent, managed); err == nil {
		t.Fatal("state directory containing managed root was accepted")
	}
	link := filepath.Join(t.TempDir(), "state-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := OpenRuntimeState(link, t.TempDir()); err == nil {
		t.Fatal("state symlink was accepted")
	}
}

func TestOpenRuntimeStateRejectsSymlinkedProspectiveOverlapBeforeCreation(t *testing.T) {
	root := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "linked-root")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	stateDir := filepath.Join(link, "state")
	if _, err := OpenRuntimeState(stateDir, root); err == nil {
		t.Fatal("state directory through managed-root symlink was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, "state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("overlapping state directory was created: %v", err)
	}
}

func TestOpenRuntimeStateRejectsCaseFoldedProspectiveOverlap(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "managed-root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	caseVariant := filepath.Join(parent, "MANAGED-ROOT")
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	variantInfo, err := os.Stat(caseVariant)
	if err != nil || !os.SameFile(rootInfo, variantInfo) {
		t.Skip("filesystem is case-sensitive")
	}
	if _, err := OpenRuntimeState(filepath.Join(caseVariant, "state"), root); err == nil {
		t.Fatal("state directory through case-folded managed root was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, "state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("overlapping state directory was created: %v", err)
	}
}

func TestOpenRuntimeStateExclusivelyLocksDirectory(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	state, err := OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRuntimeState(stateDir, root); err == nil {
		_ = state.Close()
		t.Fatal("concurrent state directory was accepted")
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStateLockExcludesOtherProcesses(t *testing.T) {
	if os.Getenv("FILEHARBOR_STATE_LOCK_HELPER") == "1" {
		lock, err := acquireStateLock(os.Getenv("FILEHARBOR_STATE_LOCK_PATH"))
		if err == nil {
			_ = lock.Close()
			os.Exit(0)
		}
		os.Exit(1)
	}

	lockPath := filepath.Join(t.TempDir(), stateLockFile)
	lock, err := acquireStateLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := runStateLockHelper(lockPath, false); err != nil {
		t.Fatalf("other process acquired active state lock: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runStateLockHelper(lockPath, true); err != nil {
		t.Fatalf("other process could not acquire released state lock: %v", err)
	}
}

func runStateLockHelper(lockPath string, wantSuccess bool) error {
	command := exec.Command(os.Args[0], "-test.run=^TestStateLockExcludesOtherProcesses$")
	command.Env = append(os.Environ(), "FILEHARBOR_STATE_LOCK_HELPER=1", "FILEHARBOR_STATE_LOCK_PATH="+lockPath)
	output, err := command.CombinedOutput()
	if wantSuccess && err != nil {
		return fmt.Errorf("helper failed: %w: %s", err, output)
	}
	if !wantSuccess && err == nil {
		return errors.New("helper unexpectedly acquired the lock")
	}
	return nil
}

func TestAuditLogWritesSafeJSONLConcurrently(t *testing.T) {
	state, err := OpenRuntimeState(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	const count = 32
	var group sync.WaitGroup
	for index := 0; index < count; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			if err := state.Record(AuditEvent{Event: "file.upload", Outcome: "success", Principal: "admin", AuthMethod: "session", Path: "reports/file.txt", Affected: index + 1}); err != nil {
				t.Errorf("record audit event: %v", err)
			}
		}(index)
	}
	group.Wait()

	file, err := os.Open(filepath.Join(state.Dir, stateAuditFile))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	seen := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		seen++
		var event AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("invalid JSONL event: %v", err)
		}
		encoded := string(scanner.Bytes())
		if event.Event != "file.upload" || event.Path != "reports/file.txt" || strings.Contains(encoded, "Authorization") || strings.Contains(encoded, "password") {
			t.Fatalf("unsafe audit event: %s", encoded)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if seen != count {
		t.Fatalf("audit events = %d, want %d", seen, count)
	}
}

func TestAuditLogRecoversTruncatedFinalRecordAndRejectsCorruption(t *testing.T) {
	root := t.TempDir()
	stateDir := t.TempDir()
	auditPath := filepath.Join(stateDir, stateAuditFile)
	valid := []byte(`{"event":"server.start","outcome":"success"}` + "\n")
	if err := os.WriteFile(auditPath, append(valid, []byte(`{"event":"partial"`)...), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := OpenRuntimeState(stateDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(auditPath)
	if err != nil || string(contents) != string(valid) {
		t.Fatalf("repaired audit log = %q, %v", contents, err)
	}

	corruptDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(corruptDir, stateAuditFile), []byte("not-json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRuntimeState(corruptDir, t.TempDir()); err == nil {
		t.Fatal("malformed audit log was accepted")
	}
}

func TestAuditRotationRetainsBoundedHistory(t *testing.T) {
	state, err := OpenRuntimeState(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()

	for generation := 0; generation < auditRetentionLogs+2; generation++ {
		fillAuditLogForRotation(t, state, generation)
		if err := state.Record(AuditEvent{Event: "server.start", Outcome: "success"}); err != nil {
			t.Fatal(err)
		}
	}

	activePath := filepath.Join(state.Dir, stateAuditFile)
	assertAuditEvent(t, activePath, "server.start")
	for index := 1; index <= auditRetentionLogs; index++ {
		rotated := fmt.Sprintf("%s.%d", activePath, index)
		assertAuditEvent(t, rotated, fmt.Sprintf("rotation-%d", auditRetentionLogs+2-index))
	}
	if _, err := os.Lstat(fmt.Sprintf("%s.%d", activePath, auditRetentionLogs+1)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired audit generation exists: %v", err)
	}
}

func fillAuditLogForRotation(t *testing.T, state *RuntimeState, generation int) {
	t.Helper()
	event := AuditEvent{Event: fmt.Sprintf("rotation-%d", generation), Outcome: "success", Path: "x"}
	oneCharacterPayload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	event.Path = strings.Repeat("x", int(maxAuditLogBytes)-len(oneCharacterPayload))
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(payload)+1) != maxAuditLogBytes {
		t.Fatalf("audit fixture size = %d, want %d", len(payload)+1, maxAuditLogBytes)
	}

	state.Audit.mu.Lock()
	defer state.Audit.mu.Unlock()
	if err := state.Audit.file.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state.Dir, stateAuditFile)
	if err := os.WriteFile(path, append(payload, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syncRuntimeDirectory(state.Dir); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	state.Audit.file = file
}

func assertAuditEvent(t *testing.T, path, wantEvent string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) == 0 || contents[len(contents)-1] != '\n' {
		t.Fatalf("audit record in %q is not newline-terminated", path)
	}
	var event AuditEvent
	if err := json.Unmarshal(contents[:len(contents)-1], &event); err != nil {
		t.Fatalf("invalid JSONL in %q: %v", path, err)
	}
	if event.Event != wantEvent {
		t.Fatalf("event in %q = %q, want %q", path, event.Event, wantEvent)
	}
}
