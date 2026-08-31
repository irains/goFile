package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/irains/fileharbor/auth"
	"github.com/irains/fileharbor/utils"
)

const (
	uploadManifestFile = "manifest.json"
	uploadPartsDir     = "parts"
	uploadSchema       = 1

	uploadStagePrefix = utils.InternalUploadStagePrefix

	uploadStateActive     = "active"
	uploadStateFinalizing = "finalizing"
	uploadStateCompleted  = "completed"
	uploadStateCancelled  = "cancelled"

	uploadIDHeader         = "X-Upload-ID"
	uploadTokenHeader      = "X-Upload-Token"
	uploadPartDigestHeader = "X-Upload-Part-SHA256"
)

var (
	errUploadNotFound      = uploadError("upload_not_found")
	errUploadConflict      = uploadError("upload_conflict")
	errUploadExpired       = uploadError("upload_expired")
	errUploadCancelled     = uploadError("upload_cancelled")
	errUploadTooLarge      = uploadError("upload_too_large")
	errUploadBusy          = uploadError("upload_busy")
	errUploadIncomplete    = uploadError("upload_incomplete")
	errUploadPartConflict  = uploadError("part_conflict")
	errUploadInvalidPart   = uploadError("invalid_part")
	errUploadInvalidDigest = uploadError("invalid_digest")
	errUploadInvalid       = uploadError("invalid_upload")
	errUploadSizeMismatch  = uploadError("size_mismatch")
	errInsufficientStorage = uploadError("insufficient_storage")
)

type uploadError string

func (err uploadError) Error() string { return string(err) }

func uploadErrorCode(err error) string {
	var public uploadError
	if errors.As(err, &public) {
		return string(public)
	}
	return utils.ErrorCode(err)
}

// UploadManifest is the durable, private record for one resumable upload. It
// intentionally stores only the hash of the client-held resume capability.
type UploadManifest struct {
	Schema         int                `json:"schema"`
	ID             string             `json:"id"`
	Owner          string             `json:"owner"`
	AuthMethod     string             `json:"auth_method"`
	TokenHash      string             `json:"token_hash"`
	Path           string             `json:"path"`
	Name           string             `json:"name"`
	Size           int64              `json:"size"`
	ChunkBytes     int64              `json:"chunk_bytes"`
	PartCount      int                `json:"part_count"`
	ExpectedSHA256 string             `json:"expected_sha256,omitempty"`
	Parts          map[int]UploadPart `json:"parts"`
	State          string             `json:"state"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	ExpiresAt      time.Time          `json:"expires_at"`
	CompletedAt    time.Time          `json:"completed_at,omitempty"`
	FinalSHA256    string             `json:"final_sha256,omitempty"`
	FinalPath      string             `json:"final_path,omitempty"`
	StageName      string             `json:"stage_name,omitempty"`
}

// UploadPart records one verified part. Files are stored by index in the
// transfer's private parts directory.
type UploadPart struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// UploadRange is an inclusive range used by status responses so large uploads
// do not need an unbounded list of received part indexes.
type UploadRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// UploadStatus is the public, token-authorized transfer view.
type UploadStatus struct {
	ID            string        `json:"id"`
	Path          string        `json:"path"`
	Name          string        `json:"name"`
	Size          int64         `json:"size"`
	ChunkBytes    int64         `json:"chunk_bytes"`
	PartCount     int           `json:"part_count"`
	Received      []UploadRange `json:"received"`
	ReceivedBytes int64         `json:"received_bytes"`
	State         string        `json:"state"`
	ExpiresAt     time.Time     `json:"expires_at"`
	FinalSHA256   string        `json:"final_sha256,omitempty"`
	FinalPath     string        `json:"final_path,omitempty"`
}

// UploadCreateRequest supplies immutable transfer metadata. Upload ID and
// resume token are headers so neither becomes part of browser history or logs.
type UploadCreateRequest struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Size           int64  `json:"size"`
	ExpectedSHA256 string `json:"sha256,omitempty"`
}

// UploadStore owns durable upload manifests, quota reservations, and per-upload
// serialization. Legacy chunks intentionally remain separate from this store.
type UploadStore struct {
	state  *RuntimeState
	config UploadConfig
	now    func() time.Time

	reapMu    sync.Mutex
	locks     [64]sync.Mutex
	partSlots chan struct{}
}

func NewUploadStore(state *RuntimeState, config UploadConfig) (*UploadStore, error) {
	if state == nil || state.UploadsDir == "" {
		return nil, errors.New("upload state is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if err := ensurePrivateDirectory(state.UploadsDir); err != nil {
		return nil, err
	}
	store := &UploadStore{
		state:     state,
		config:    config,
		now:       time.Now,
		partSlots: make(chan struct{}, config.MaxConcurrentParts),
	}
	if err := store.Recover(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *UploadStore) StartReaper(done <-chan struct{}) {
	if store == nil || done == nil {
		return
	}
	interval := store.config.InactivityTTL / 12
	if interval < time.Minute {
		interval = time.Minute
	}
	if interval > time.Hour {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = store.ReapExpired()
			}
		}
	}()
}

func (store *UploadStore) Recover() error {
	if store == nil {
		return errors.New("upload store unavailable")
	}
	store.reapMu.Lock()
	defer store.reapMu.Unlock()
	return store.recoverLocked()
}

func (store *UploadStore) recoverLocked() error {
	entries, err := os.ReadDir(store.state.UploadsDir)
	if err != nil {
		return errors.New("could not inspect upload state")
	}
	for _, entry := range entries {
		if !validUploadID(entry.Name()) || !entry.IsDir() {
			return errors.New("upload state contains an invalid entry")
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("upload state contains an unsafe entry")
		}
		manifest, err := store.loadManifest(entry.Name())
		if err != nil {
			return err
		}
		if err := store.recoverManifestLocked(manifest); err != nil {
			return err
		}
	}
	return store.reapExpiredLocked(store.now())
}

func (store *UploadStore) cleanupCompleted(manifest *UploadManifest) error {
	if manifest == nil || manifest.State != uploadStateCompleted {
		return errors.New("completed upload manifest is invalid")
	}
	partsDir := filepath.Join(store.uploadPath(manifest.ID), uploadPartsDir)
	if info, err := os.Lstat(partsDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("completed upload parts are unsafe")
		}
		if err := os.RemoveAll(partsDir); err != nil {
			return errors.New("could not remove completed upload parts")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return errors.New("could not inspect completed upload parts")
	}
	return nil
}

func (store *UploadStore) cleanupCancelled(manifest *UploadManifest) error {
	if manifest == nil || manifest.State != uploadStateCancelled {
		return errors.New("cancelled upload manifest is invalid")
	}
	partsDir := filepath.Join(store.uploadPath(manifest.ID), uploadPartsDir)
	if info, err := os.Lstat(partsDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("cancelled upload parts are unsafe")
		}
		if err := os.RemoveAll(partsDir); err != nil {
			return errors.New("could not remove cancelled upload parts")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return errors.New("could not inspect cancelled upload parts")
	}
	return nil
}

func (store *UploadStore) recoverManifestLocked(manifest *UploadManifest) error {
	if manifest == nil {
		return errors.New("upload manifest is unavailable")
	}
	switch manifest.State {
	case uploadStateCompleted:
		return store.cleanupCompleted(manifest)
	case uploadStateCancelled:
		return store.cleanupCancelled(manifest)
	}
	if err := store.verifyUploadParts(manifest); err != nil {
		return err
	}
	if manifest.State != uploadStateFinalizing {
		return nil
	}
	return store.recoverFinalizing(manifest)
}

// recoverFinalizing reconciles the only ambiguous durable state: publication
// could have succeeded after the stage and manifest were synced, but before the
// completed manifest was saved. Private parts are the durable source of truth
// for the final digest, so an intact matching destination is safe to adopt.
func (store *UploadStore) recoverFinalizing(manifest *UploadManifest) error {
	if manifest == nil || manifest.State != uploadStateFinalizing {
		return errors.New("upload finalization state is invalid")
	}
	finalDigest, err := store.partsSHA256(manifest)
	if err != nil {
		return err
	}
	if manifest.ExpectedSHA256 != "" && manifest.ExpectedSHA256 != finalDigest {
		return errors.New("upload finalization parts checksum mismatch")
	}
	directory, relative, _, err := utils.ResolveDirectory(manifest.Path, true)
	if err != nil {
		return errors.New("could not recover upload destination")
	}
	destination := filepath.Join(directory, manifest.Name)
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() || info.Size() != manifest.Size {
			return errors.New("upload finalization has an unexpected destination")
		}
		digest, hashErr := fileSHA256(destination)
		if hashErr != nil || digest != finalDigest {
			return errors.New("upload finalization destination checksum mismatch")
		}
		manifest.FinalSHA256 = finalDigest
		return store.markCompleted(manifest, filepath.ToSlash(filepath.Join(relative, manifest.Name)))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return errors.New("could not inspect upload destination")
	}

	// The crash occurred before publication. Discard the reserved stage and let a
	// future complete request assemble verified private parts again.
	stage, err := store.stagePath(manifest)
	if err != nil {
		return err
	}
	if err := removeRegularFile(stage); err != nil {
		return errors.New("could not recover upload staging file")
	}
	manifest.State = uploadStateActive
	manifest.StageName = ""
	manifest.FinalSHA256 = ""
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	return store.saveManifest(manifest)
}

func (store *UploadStore) Create(info auth.Info, id, token string, request UploadCreateRequest) (*UploadStatus, bool, error) {
	owner, method := uploadOwner(info)
	if owner == "" || !validUploadID(id) || !validUploadToken(token) {
		return nil, false, errUploadNotFound
	}
	if err := store.validateCreate(&request); err != nil {
		return nil, false, err
	}

	// The reaper and creation take locks in the same order. Keeping the reaper
	// lock through the directory creation prevents it from mistaking a partial
	// transfer directory for stale state.
	store.reapMu.Lock()
	defer store.reapMu.Unlock()
	if err := store.reapExpiredLocked(store.now()); err != nil {
		return nil, false, err
	}
	lock := store.lockFor(id)
	lock.Lock()
	defer lock.Unlock()

	path := store.uploadPath(id)
	if _, err := os.Lstat(path); err == nil {
		manifest, loadErr := store.loadManifest(id)
		if loadErr != nil {
			return nil, false, loadErr
		}
		if !manifestAuthorized(manifest, owner, method, token) {
			return nil, false, errUploadNotFound
		}
		if !sameCreateRequest(manifest, request) {
			return nil, false, errUploadConflict
		}
		status := manifestStatus(manifest)
		return &status, false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, false, errors.New("could not inspect upload state")
	}
	active, pending, err := store.reservationsLocked()
	if err != nil {
		return nil, false, err
	}
	if active >= store.config.MaxActive || pending > store.config.MaxPendingBytes-request.Size {
		return nil, false, errUploadBusy
	}
	if total, free := utils.DiskUsage(store.state.UploadsDir); total > 0 && free < uint64(request.Size)+uint64(store.config.MinFreeBytes) {
		return nil, false, errInsufficientStorage
	}
	if err := os.Mkdir(path, 0700); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, false, errUploadConflict
		}
		return nil, false, errors.New("could not create upload state")
	}
	if err := ensurePrivateDirectory(filepath.Join(path, uploadPartsDir)); err != nil {
		_ = os.RemoveAll(path)
		return nil, false, err
	}
	now := store.now().UTC()
	manifest := &UploadManifest{
		Schema:         uploadSchema,
		ID:             id,
		Owner:          owner,
		AuthMethod:     method,
		TokenHash:      tokenHash(token),
		Path:           request.Path,
		Name:           request.Name,
		Size:           request.Size,
		ChunkBytes:     store.config.ChunkBytes,
		PartCount:      partCount(request.Size, store.config.ChunkBytes),
		ExpectedSHA256: request.ExpectedSHA256,
		Parts:          make(map[int]UploadPart),
		State:          uploadStateActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      now.Add(store.config.InactivityTTL),
	}
	if err := store.saveManifest(manifest); err != nil {
		_ = os.RemoveAll(path)
		return nil, false, err
	}
	status := manifestStatus(manifest)
	return &status, true, nil
}

func (store *UploadStore) Status(info auth.Info, id, token string) (*UploadStatus, error) {
	manifest, lock, err := store.authorizedManifest(info, id, token)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if store.expired(manifest, store.now()) {
		return nil, errUploadExpired
	}
	status := manifestStatus(manifest)
	return &status, nil
}

func (store *UploadStore) WritePart(info auth.Info, id, token string, index int, digest string, body io.Reader) (*UploadStatus, bool, error) {
	if !validSHA256(digest) {
		return nil, false, errUploadInvalidDigest
	}
	manifest, lock, err := store.authorizedManifest(info, id, token)
	if err != nil {
		return nil, false, err
	}
	defer lock.Unlock()
	if store.expired(manifest, store.now()) {
		return nil, false, errUploadExpired
	}
	if manifest.State == uploadStateCancelled {
		return nil, false, errUploadCancelled
	}
	if manifest.State == uploadStateCompleted {
		return nil, false, errUploadConflict
	}
	if manifest.State != uploadStateActive || index < 0 || index >= manifest.PartCount {
		return nil, false, errUploadInvalidPart
	}
	expected := expectedPartSize(manifest, index)
	if part, exists := manifest.Parts[index]; exists {
		if part.Size == expected && subtle.ConstantTimeCompare([]byte(part.SHA256), []byte(strings.ToLower(digest))) == 1 {
			status := manifestStatus(manifest)
			return &status, true, nil
		}
		return nil, false, errUploadPartConflict
	}
	select {
	case store.partSlots <- struct{}{}:
		defer func() { <-store.partSlots }()
	default:
		return nil, false, errUploadBusy
	}
	partPath, err := store.partPath(manifest, index)
	if err != nil {
		return nil, false, err
	}
	// A crash after a part rename but before the manifest update leaves an
	// unrecorded regular file. It is safe to replace on retry, but a link or any
	// other entry in this private directory is treated as corrupted state.
	if err := removeRegularFile(partPath); err != nil {
		return nil, false, err
	}
	temporary := partPath + ".tmp"
	if err := removeRegularFile(temporary); err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, false, errors.New("could not create upload part")
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(body, expected+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written != expected {
		_ = os.Remove(temporary)
		if written > expected {
			return nil, false, errUploadSizeMismatch
		}
		if copyErr != nil || syncErr != nil || closeErr != nil {
			if errors.As(copyErr, new(*http.MaxBytesError)) {
				return nil, false, fmt.Errorf("%w: %w", errUploadSizeMismatch, copyErr)
			}
			return nil, false, errors.New("could not write upload part")
		}
		return nil, false, errUploadSizeMismatch
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(actualDigest), []byte(strings.ToLower(digest))) != 1 {
		_ = os.Remove(temporary)
		return nil, false, errUploadInvalidDigest
	}
	if err := os.Rename(temporary, partPath); err != nil {
		_ = os.Remove(temporary)
		return nil, false, errors.New("could not publish upload part")
	}
	if err := syncRuntimeDirectory(filepath.Dir(partPath)); err != nil {
		return nil, false, errors.New("could not sync upload part directory")
	}
	manifest.Parts[index] = UploadPart{Size: written, SHA256: actualDigest}
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		return nil, false, err
	}
	status := manifestStatus(manifest)
	return &status, false, nil
}

func (store *UploadStore) Complete(info auth.Info, id, token string) (*UploadStatus, bool, error) {
	manifest, lock, err := store.authorizedManifest(info, id, token)
	if err != nil {
		return nil, false, err
	}
	defer lock.Unlock()
	if store.expired(manifest, store.now()) {
		return nil, false, errUploadExpired
	}
	if manifest.State == uploadStateCancelled {
		return nil, false, errUploadCancelled
	}
	if manifest.State == uploadStateCompleted {
		status := manifestStatus(manifest)
		return &status, true, nil
	}
	if manifest.State == uploadStateFinalizing {
		if err := store.recoverFinalizing(manifest); err != nil {
			return nil, false, err
		}
		if manifest.State == uploadStateCompleted {
			status := manifestStatus(manifest)
			return &status, true, nil
		}
	}
	if manifest.State != uploadStateActive || len(manifest.Parts) != manifest.PartCount {
		return nil, false, errUploadIncomplete
	}
	if err := store.publish(manifest); err != nil {
		return nil, false, err
	}
	status := manifestStatus(manifest)
	return &status, false, nil
}

func (store *UploadStore) Cancel(info auth.Info, id, token string) error {
	manifest, lock, err := store.authorizedManifest(info, id, token)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	if store.expired(manifest, store.now()) {
		return errUploadExpired
	}
	if manifest.State == uploadStateCompleted {
		return errUploadConflict
	}
	if manifest.State == uploadStateFinalizing {
		if err := store.recoverFinalizing(manifest); err != nil {
			return err
		}
		if manifest.State == uploadStateCompleted {
			return errUploadConflict
		}
	}
	if manifest.State == uploadStateCancelled {
		return nil
	}
	if manifest.StageName != "" {
		stage, err := store.stagePath(manifest)
		if err != nil {
			return err
		}
		if err := removeRegularFile(stage); err != nil {
			return errors.New("could not remove upload staging file")
		}
	}
	manifest.State = uploadStateCancelled
	manifest.StageName = ""
	manifest.FinalSHA256 = ""
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.CompletionTTL)
	if err := store.saveManifest(manifest); err != nil {
		return err
	}
	if err := store.cleanupCancelled(manifest); err != nil {
		return err
	}
	return syncRuntimeDirectory(store.uploadPath(id))
}

func (store *UploadStore) ReapExpired() error {
	if store == nil {
		return nil
	}
	store.reapMu.Lock()
	defer store.reapMu.Unlock()
	return store.reapExpiredLocked(store.now())
}

func (store *UploadStore) reapExpiredLocked(now time.Time) error {
	entries, err := os.ReadDir(store.state.UploadsDir)
	if err != nil {
		return errors.New("could not inspect upload state")
	}
	for _, entry := range entries {
		if !validUploadID(entry.Name()) || !entry.IsDir() {
			return errors.New("upload state contains an invalid entry")
		}
		lock := store.lockFor(entry.Name())
		lock.Lock()
		manifest, err := store.loadManifest(entry.Name())
		if err != nil {
			lock.Unlock()
			return err
		}
		if store.expired(manifest, now) {
			if err := os.RemoveAll(store.uploadPath(manifest.ID)); err != nil {
				lock.Unlock()
				return errors.New("could not expire upload state")
			}
			if err := syncRuntimeDirectory(store.state.UploadsDir); err != nil {
				lock.Unlock()
				return errors.New("could not sync upload state")
			}
		}
		lock.Unlock()
	}
	return nil
}

func (store *UploadStore) authorizedManifest(info auth.Info, id, token string) (*UploadManifest, *sync.Mutex, error) {
	owner, method := uploadOwner(info)
	if owner == "" || !validUploadID(id) || !validUploadToken(token) {
		return nil, nil, errUploadNotFound
	}
	lock := store.lockFor(id)
	lock.Lock()
	manifest, err := store.loadManifest(id)
	if err != nil {
		lock.Unlock()
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errUploadNotFound
		}
		return nil, nil, err
	}
	if !manifestAuthorized(manifest, owner, method, token) {
		lock.Unlock()
		return nil, nil, errUploadNotFound
	}
	return manifest, lock, nil
}

func (store *UploadStore) lockFor(id string) *sync.Mutex {
	var hash uint32 = 2166136261
	for _, value := range id {
		hash ^= uint32(value)
		hash *= 16777619
	}
	return &store.locks[hash%uint32(len(store.locks))]
}

func (store *UploadStore) validateCreate(request *UploadCreateRequest) error {
	if request == nil || request.Size < 0 || request.Size > store.config.MaxFileBytes {
		return errUploadTooLarge
	}
	if utils.ValidateLeafName(request.Name) != nil {
		return errUploadInvalid
	}
	path, err := utils.CleanRelative(request.Path, true)
	if err != nil {
		return errUploadInvalid
	}
	request.Path = path
	request.ExpectedSHA256 = normalizedDigest(request.ExpectedSHA256)
	if request.ExpectedSHA256 != "" && !validSHA256(request.ExpectedSHA256) {
		return errUploadInvalidDigest
	}
	if partCount(request.Size, store.config.ChunkBytes) > store.config.MaxParts {
		return errUploadTooLarge
	}
	return nil
}

func (store *UploadStore) reservationsLocked() (int, int64, error) {
	entries, err := os.ReadDir(store.state.UploadsDir)
	if err != nil {
		return 0, 0, errors.New("could not inspect upload state")
	}
	var active int
	var pending int64
	for _, entry := range entries {
		if !validUploadID(entry.Name()) || !entry.IsDir() {
			return 0, 0, errors.New("upload state contains an invalid entry")
		}
		manifest, err := store.loadManifest(entry.Name())
		if err != nil {
			return 0, 0, err
		}
		if manifest.State == uploadStateActive || manifest.State == uploadStateFinalizing {
			active++
			if pending > store.config.MaxPendingBytes-manifest.Size {
				return active, store.config.MaxPendingBytes + 1, nil
			}
			pending += manifest.Size
		}
	}
	return active, pending, nil
}

func (store *UploadStore) loadManifest(id string) (*UploadManifest, error) {
	path := filepath.Join(store.uploadPath(id), uploadManifestFile)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("upload manifest is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not read upload manifest")
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20+1))
	decoder.DisallowUnknownFields()
	var manifest UploadManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, errors.New("upload manifest is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("upload manifest is invalid")
	}
	if err := store.validateManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (store *UploadStore) saveManifest(manifest *UploadManifest) error {
	if err := store.validateManifest(manifest); err != nil {
		return err
	}
	dir := store.uploadPath(manifest.ID)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return errors.New("could not encode upload manifest")
	}
	temporary, err := os.CreateTemp(dir, ".manifest-*")
	if err != nil {
		return errors.New("could not write upload manifest")
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(encoded); err != nil || temporary.Sync() != nil || temporary.Close() != nil {
		_ = temporary.Close()
		return errors.New("could not write upload manifest")
	}
	if err := protectPrivateFile(temporaryName); err != nil {
		return errors.New("could not protect upload manifest")
	}
	if err := os.Rename(temporaryName, filepath.Join(dir, uploadManifestFile)); err != nil {
		return errors.New("could not publish upload manifest")
	}
	if err := syncRuntimeDirectory(dir); err != nil {
		return errors.New("could not sync upload manifest")
	}
	return nil
}

func (store *UploadStore) validateManifest(manifest *UploadManifest) error {
	if manifest == nil || manifest.Schema != uploadSchema || !validUploadID(manifest.ID) ||
		manifest.Owner == "" || (manifest.AuthMethod != "session" && manifest.AuthMethod != "bearer") ||
		!validSHA256(manifest.TokenHash) || utils.ValidateLeafName(manifest.Name) != nil ||
		manifest.Size < 0 || manifest.Size > store.config.MaxFileBytes ||
		manifest.ChunkBytes != store.config.ChunkBytes || manifest.PartCount != partCount(manifest.Size, manifest.ChunkBytes) ||
		manifest.PartCount < 1 || manifest.PartCount > store.config.MaxParts || manifest.Parts == nil ||
		!validUploadState(manifest.State) || manifest.CreatedAt.IsZero() || manifest.UpdatedAt.IsZero() || manifest.ExpiresAt.IsZero() {
		return errors.New("upload manifest is invalid")
	}
	if cleaned, err := utils.CleanRelative(manifest.Path, true); err != nil || cleaned != manifest.Path {
		return errors.New("upload manifest has an invalid destination")
	}
	if manifest.ExpectedSHA256 != "" && !validSHA256(manifest.ExpectedSHA256) {
		return errors.New("upload manifest has an invalid checksum")
	}
	if manifest.FinalSHA256 != "" && !validSHA256(manifest.FinalSHA256) {
		return errors.New("upload manifest has an invalid final checksum")
	}
	switch manifest.State {
	case uploadStateActive:
		if manifest.StageName != "" || manifest.FinalSHA256 != "" || manifest.FinalPath != "" || !manifest.CompletedAt.IsZero() {
			return errors.New("active upload manifest is invalid")
		}
	case uploadStateFinalizing:
		if !isUploadStageName(manifest.StageName) || manifest.FinalPath != "" || !manifest.CompletedAt.IsZero() {
			return errors.New("finalizing upload manifest is invalid")
		}
	case uploadStateCancelled:
		if manifest.StageName != "" || manifest.FinalSHA256 != "" || manifest.FinalPath != "" || !manifest.CompletedAt.IsZero() {
			return errors.New("cancelled upload manifest is invalid")
		}
	case uploadStateCompleted:
		if manifest.StageName != "" || manifest.FinalPath == "" || manifest.FinalSHA256 == "" || manifest.CompletedAt.IsZero() || len(manifest.Parts) != 0 {
			return errors.New("completed upload manifest is invalid")
		}
	}
	if manifest.State != uploadStateCompleted && manifest.State != uploadStateCancelled {
		for index, part := range manifest.Parts {
			if index < 0 || index >= manifest.PartCount || part.Size != expectedPartSize(manifest, index) || !validSHA256(part.SHA256) {
				return errors.New("upload manifest has an invalid part")
			}
		}
	}
	return nil
}

func (store *UploadStore) partsSHA256(manifest *UploadManifest) (string, error) {
	hash := sha256.New()
	var written int64
	for index := 0; index < manifest.PartCount; index++ {
		part, ok := manifest.Parts[index]
		if !ok {
			return "", errUploadIncomplete
		}
		path, err := store.partPath(manifest, index)
		if err != nil {
			return "", err
		}
		file, err := safeUploadPart(path, part)
		if err != nil {
			return "", err
		}
		copied, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil || copied != part.Size {
			return "", errors.New("could not verify upload parts")
		}
		written += copied
	}
	if written != manifest.Size {
		return "", errUploadSizeMismatch
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (store *UploadStore) verifyUploadParts(manifest *UploadManifest) error {
	for index, part := range manifest.Parts {
		path, err := store.partPath(manifest, index)
		if err != nil {
			return err
		}
		file, err := safeUploadPart(path, part)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return errors.New("could not read upload part")
		}
	}
	return nil
}

func (store *UploadStore) uploadPath(id string) string {
	return filepath.Join(store.state.UploadsDir, id)
}

func (store *UploadStore) partPath(manifest *UploadManifest, index int) (string, error) {
	if manifest == nil || index < 0 || index >= manifest.PartCount {
		return "", errUploadInvalidPart
	}
	return filepath.Join(store.uploadPath(manifest.ID), uploadPartsDir, fmt.Sprintf("%010d", index)), nil
}

func (store *UploadStore) stagePath(manifest *UploadManifest) (string, error) {
	if manifest == nil || manifest.StageName == "" || !isUploadStageName(manifest.StageName) {
		return "", errors.New("upload staging state is invalid")
	}
	directory, _, _, err := utils.ResolveDirectory(manifest.Path, true)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, manifest.StageName), nil
}

func (store *UploadStore) expired(manifest *UploadManifest, now time.Time) bool {
	return !now.Before(manifest.ExpiresAt)
}

func validUploadID(value string) bool {
	return len(value) >= 16 && len(value) <= 128 && validChunkID(value)
}

func validUploadToken(value string) bool {
	return len(value) == 64 && validSHA256(value)
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func normalizedDigest(value string) string {
	return strings.ToLower(value)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func uploadOwner(info auth.Info) (string, string) {
	if info.Bearer {
		return "api-token", "bearer"
	}
	if info.Username != "" && info.SessionID != "" {
		return info.Username, "session"
	}
	return "", ""
}

func manifestAuthorized(manifest *UploadManifest, owner, method, token string) bool {
	return manifest != nil && manifest.Owner == owner && manifest.AuthMethod == method &&
		subtle.ConstantTimeCompare([]byte(manifest.TokenHash), []byte(tokenHash(token))) == 1
}

func sameCreateRequest(manifest *UploadManifest, request UploadCreateRequest) bool {
	path, err := utils.CleanRelative(request.Path, true)
	return err == nil && manifest.Path == path && manifest.Name == request.Name && manifest.Size == request.Size &&
		manifest.ExpectedSHA256 == normalizedDigest(request.ExpectedSHA256)
}

func partCount(size, chunkBytes int64) int {
	if chunkBytes <= 0 || size < 0 {
		return 0
	}
	if size == 0 {
		return 1
	}
	return int((size + chunkBytes - 1) / chunkBytes)
}

func expectedPartSize(manifest *UploadManifest, index int) int64 {
	if manifest.Size == 0 {
		return 0
	}
	if index < manifest.PartCount-1 {
		return manifest.ChunkBytes
	}
	remaining := manifest.Size - int64(index)*manifest.ChunkBytes
	if remaining == 0 {
		return manifest.ChunkBytes
	}
	return remaining
}

func validUploadState(state string) bool {
	switch state {
	case uploadStateActive, uploadStateFinalizing, uploadStateCompleted, uploadStateCancelled:
		return true
	default:
		return false
	}
}

func receivedRanges(parts map[int]UploadPart) []UploadRange {
	if len(parts) == 0 {
		return []UploadRange{}
	}
	indexes := make([]int, 0, len(parts))
	for index := range parts {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	ranges := make([]UploadRange, 0)
	for _, index := range indexes {
		if len(ranges) == 0 || index != ranges[len(ranges)-1].End+1 {
			ranges = append(ranges, UploadRange{Start: index, End: index})
			continue
		}
		ranges[len(ranges)-1].End = index
	}
	return ranges
}

func manifestStatus(manifest *UploadManifest) UploadStatus {
	var receivedBytes int64
	for _, part := range manifest.Parts {
		receivedBytes += part.Size
	}
	return UploadStatus{
		ID:            manifest.ID,
		Path:          manifest.Path,
		Name:          manifest.Name,
		Size:          manifest.Size,
		ChunkBytes:    manifest.ChunkBytes,
		PartCount:     manifest.PartCount,
		Received:      receivedRanges(manifest.Parts),
		ReceivedBytes: receivedBytes,
		State:         manifest.State,
		ExpiresAt:     manifest.ExpiresAt,
		FinalSHA256:   manifest.FinalSHA256,
		FinalPath:     manifest.FinalPath,
	}
}

func removeRegularFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("upload temporary file is unsafe")
	}
	if err := os.Remove(path); err != nil {
		return errors.New("could not remove upload temporary file")
	}
	return nil
}

func isUploadStageName(name string) bool {
	if !strings.HasPrefix(name, uploadStagePrefix) || !strings.HasSuffix(name, ".partial") {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, uploadStagePrefix), ".partial")
	return validUploadID(id)
}

func uploadStageName(id string) string {
	return uploadStagePrefix + id + ".partial"
}

func uploadPartFilename(index int) string {
	return fmt.Sprintf("%010d", index)
}

func parsePartIndex(raw string) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 || strconv.Itoa(index) != raw {
		return 0, errUploadInvalidPart
	}
	return index, nil
}
