package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/irains/fileharbor/utils"
)

// publish assembles verified private parts into a hidden same-directory stage,
// then publishes it without replacing an existing user file.
func (store *UploadStore) publish(manifest *UploadManifest) error {
	if manifest == nil || manifest.State != uploadStateActive {
		return errUploadConflict
	}
	directory, rel, _, err := utils.ResolveDirectory(manifest.Path, true)
	if err != nil {
		return errUploadConflict
	}
	destination := filepath.Join(directory, manifest.Name)
	if info, err := os.Lstat(destination); err == nil {
		if info.Mode().IsRegular() && manifest.FinalSHA256 != "" && info.Size() == manifest.Size {
			if digest, hashErr := fileSHA256(destination); hashErr == nil && digest == manifest.FinalSHA256 {
				return store.markCompleted(manifest, filepath.ToSlash(filepath.Join(rel, manifest.Name)))
			}
		}
		return utils.ErrDestinationExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return errors.New("could not inspect upload destination")
	}
	if total, free := utils.DiskUsage(directory); total > 0 && free < uint64(manifest.Size)+uint64(store.config.MinFreeBytes) {
		return errInsufficientStorage
	}

	manifest.State = uploadStateFinalizing
	manifest.StageName = uploadStageName(manifest.ID)
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		return err
	}
	stage, err := store.stagePath(manifest)
	if err != nil {
		return err
	}
	reset := func(cause error) error {
		if resetErr := store.resetFinalizing(manifest, stage); resetErr != nil {
			return resetErr
		}
		return cause
	}
	if err := removeRegularFile(stage); err != nil {
		return reset(err)
	}
	file, err := os.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return reset(errors.New("could not create upload staging file"))
	}
	hash := sha256.New()
	var written int64
	for index := 0; index < manifest.PartCount; index++ {
		part, ok := manifest.Parts[index]
		if !ok {
			_ = file.Close()
			return reset(errUploadIncomplete)
		}
		partPath, pathErr := store.partPath(manifest, index)
		if pathErr != nil {
			_ = file.Close()
			return reset(pathErr)
		}
		input, openErr := safeUploadPart(partPath, part)
		if openErr != nil {
			_ = file.Close()
			return reset(openErr)
		}
		copied, copyErr := io.Copy(io.MultiWriter(file, hash), input)
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil || copied != part.Size {
			_ = file.Close()
			return reset(errors.New("could not assemble upload"))
		}
		written += copied
	}
	if written != manifest.Size || file.Sync() != nil || file.Chmod(0644) != nil || file.Close() != nil {
		_ = file.Close()
		return reset(errUploadSizeMismatch)
	}
	finalDigest := hex.EncodeToString(hash.Sum(nil))
	if manifest.ExpectedSHA256 != "" && manifest.ExpectedSHA256 != finalDigest {
		return reset(errUploadInvalidDigest)
	}
	manifest.FinalSHA256 = finalDigest
	manifest.UpdatedAt = store.now().UTC()
	if err := store.saveManifest(manifest); err != nil {
		return reset(err)
	}
	// A same-directory hard link creates the requested name atomically and fails
	// rather than replacing a file created by another operation.
	if err := os.Link(stage, destination); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return reset(utils.ErrDestinationExists)
		}
		return reset(errors.New("could not publish upload"))
	}
	if err := syncRuntimeDirectory(directory); err != nil {
		return errors.New("could not sync upload destination")
	}
	return store.markCompleted(manifest, filepath.ToSlash(filepath.Join(rel, manifest.Name)))
}

func (store *UploadStore) resetFinalizing(manifest *UploadManifest, stage string) error {
	if manifest == nil || manifest.State != uploadStateFinalizing {
		return errors.New("upload finalization state is invalid")
	}
	if err := removeRegularFile(stage); err != nil {
		return errors.New("could not remove upload staging file")
	}
	manifest.State = uploadStateActive
	manifest.StageName = ""
	manifest.FinalSHA256 = ""
	manifest.UpdatedAt = store.now().UTC()
	manifest.ExpiresAt = manifest.UpdatedAt.Add(store.config.InactivityTTL)
	if err := store.saveManifest(manifest); err != nil {
		return err
	}
	return syncRuntimeDirectory(filepath.Dir(stage))
}

func (store *UploadStore) markCompleted(manifest *UploadManifest, finalPath string) error {
	if manifest == nil {
		return errors.New("upload manifest is unavailable")
	}
	stage := ""
	if manifest.StageName != "" {
		var err error
		stage, err = store.stagePath(manifest)
		if err != nil {
			return err
		}
	}
	manifest.State = uploadStateCompleted
	manifest.FinalPath = finalPath
	manifest.StageName = ""
	manifest.Parts = make(map[int]UploadPart)
	manifest.CompletedAt = store.now().UTC()
	manifest.UpdatedAt = manifest.CompletedAt
	manifest.ExpiresAt = manifest.CompletedAt.Add(store.config.CompletionTTL)
	if err := store.saveManifest(manifest); err != nil {
		return err
	}
	if err := store.cleanupCompleted(manifest); err != nil {
		return err
	}
	if stage != "" {
		if err := os.Remove(stage); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return errors.New("could not remove completed upload staging file")
		}
	}
	return syncRuntimeDirectory(store.uploadPath(manifest.ID))
}

func safeUploadPart(path string, expected UploadPart) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != expected.Size {
		return nil, errors.New("upload part is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not read upload part")
	}
	hash := sha256.New()
	copied, copyErr := io.Copy(hash, io.LimitReader(file, expected.Size+1))
	if copyErr != nil || copied != expected.Size {
		_ = file.Close()
		return nil, errors.New("could not verify upload part")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, errors.New("could not read upload part")
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
		_ = file.Close()
		return nil, errors.New("upload part checksum mismatch")
	}
	return file, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
