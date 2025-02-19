// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
)

const (
	blobPermission = 0600 // matches os.CreateTemp
	dirPermission  = 0700

	v0PieceFileSuffix      = ""
	v1PieceFileSuffix      = ".sj1"
	unknownPieceFileSuffix = "/..error_unknown_format../"
	verificationFileName   = "storage-dir-verification"

	// TrashUsesDayDirsIndicator is the name of a file whose presence under
	// trashdir indicates per-day directories can be used. absence of this file
	// means there is still trash in trash/$namespace/?? directories that needs
	// to be migrated to per-day directories.
	TrashUsesDayDirsIndicator = ".trash-uses-day-dirs-indicator"
)

// PathEncoding is the encoding used for the namespace and key in the filestore.
var PathEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// Dir represents single folder for storing blobs.
type Dir struct {
	log  *zap.Logger
	path string

	// blobsdir is the sub-directory containing the blobs.
	blobsdir string
	// tempdir is used for temp files prior to being moved into blobsdir.
	tempdir string
	// trashdir contains files staged for deletion for a period of time.
	trashdir string
	info     *DirSpaceInfo
}

// OpenDir opens existing folder for storing blobs.
func OpenDir(log *zap.Logger, path string, now time.Time) (*Dir, error) {
	dir := &Dir{log: log}
	dir.setPath(path)
	dir.info = NewDirSpaceInfo(path)

	stat := func(path string) error {
		_, err := os.Stat(path)
		return err
	}
	err := errs.Combine(
		stat(dir.blobsdir),
		stat(dir.tempdir),
		stat(dir.trashdir),
	)
	if err != nil {
		return nil, err
	}

	indicatorFile := filepath.Join(dir.trashdir, TrashUsesDayDirsIndicator)
	if stat(indicatorFile) != nil {
		err = dir.migrateTrashToPerDayDirs(now)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(indicatorFile, []byte("do not delete this file"), 0644)
		if err != nil {
			return nil, err
		}
	}

	return dir, nil
}

// NewDir returns folder for storing blobs.
func NewDir(log *zap.Logger, path string) (dir *Dir, err error) {
	dir = &Dir{
		log:  log,
		info: NewDirSpaceInfo(path),
	}
	dir.setPath(path)

	err = errs.Combine(
		os.MkdirAll(dir.blobsdir, dirPermission),
		os.MkdirAll(dir.tempdir, dirPermission),
		os.MkdirAll(dir.trashdir, dirPermission),
	)
	if err != nil {
		return nil, err
	}

	// this should fail if the file already exists; thus, O_EXCL, and we can't use os.WriteFile for it
	f, err := os.OpenFile(filepath.Join(dir.trashdir, TrashUsesDayDirsIndicator), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, f.Close())
	}()
	_, err = f.WriteString("do not delete this file")
	return dir, err
}

// Path returns the directory path.
func (dir *Dir) Path() string { return dir.path }

func (dir *Dir) setPath(path string) {
	dir.path = path

	dir.blobsdir = filepath.Join(path, "blobs")
	dir.tempdir = filepath.Join(path, "temp")
	dir.trashdir = filepath.Join(path, "trash")
}

// trashPath returns the toplevel trash directory for the given namespace and timestamp.
func (dir *Dir) trashPath(namespace []byte, forTime time.Time) string {
	namespaceStr := PathEncoding.EncodeToString(namespace)
	dayDirName := forTime.UTC().Format("2006-01-02")
	return filepath.Join(dir.trashdir, namespaceStr, dayDirName)
}

// refToTrashPath converts a blob reference to a filepath in the trash hierarchy with the given timestamp.
func (dir *Dir) refToTrashPath(ref blobstore.BlobRef, forTime time.Time) (string, error) {
	if !ref.IsValid() {
		return "", blobstore.ErrInvalidBlobRef.New("")
	}

	r := make([]byte, 0, len(dir.trashdir)+trashSubdirLength(ref.Namespace)+encodedKeyPathLen(ref.Key))
	r = append(r, []byte(dir.trashdir)...)
	r = appendTrashSubdir(r, ref.Namespace, forTime)
	r = appendEncodedKeyPath(r, ref.Key)

	return unsafe.String(&r[0], len(r)), nil // using unsafe.String here to avoid an allocations.
}

// CreateVerificationFile creates a file to be used for storage directory verification.
func (dir *Dir) CreateVerificationFile(ctx context.Context, id storj.NodeID) error {
	f, err := os.Create(filepath.Join(dir.path, verificationFileName))
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, f.Close())
	}()
	_, err = f.Write(id.Bytes())
	return err
}

// Verify verifies that the storage directory is correct by checking for the existence and validity
// of the verification file.
func (dir *Dir) Verify(ctx context.Context, id storj.NodeID) error {
	content, err := os.ReadFile(filepath.Join(dir.path, verificationFileName))
	if err != nil {
		return err
	}

	if !bytes.Equal(content, id.Bytes()) {
		verifyID, err := storj.NodeIDFromBytes(content)
		if err != nil {
			return errs.New("content of file is not a valid node ID: %x", content)
		}
		return errs.New("node ID in file (%s) does not match running node's ID (%s)", verifyID, id.String())
	}
	return nil
}

// CreateTemporaryFile creates a preallocated temporary file in the temp directory.
func (dir *Dir) CreateTemporaryFile(ctx context.Context) (_ *os.File, err error) {
	file, err := os.CreateTemp(dir.tempdir, "blob-*.partial")
	if err != nil {
		return nil, err
	}
	return file, nil
}

// CreateNamedFile creates a preallocated file in the correct destination directory.
func (dir *Dir) CreateNamedFile(ref blobstore.BlobRef, formatVersion blobstore.FormatVersion) (file *os.File, err error) {
	path, err := dir.blobToBasePath(ref)
	if err != nil {
		return nil, err
	}
	path = blobPathForFormatVersion(path, formatVersion)

	file, err = os.Create(path)
	if err != nil {
		mkdirErr := os.MkdirAll(filepath.Dir(path), dirPermission)
		if mkdirErr != nil {
			return nil, Error.Wrap(errs.Combine(err, mkdirErr))
		}
		file, err = os.Create(path)
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

// DeleteTemporary deletes a temporary file.
func (dir *Dir) DeleteTemporary(ctx context.Context, file *os.File) (err error) {
	defer mon.Task()(&ctx)(&err)
	closeErr := file.Close()
	return errs.Combine(closeErr, os.Remove(file.Name()))
}

// blobToBasePath converts a blob reference to a filepath in permanent storage. This may not be the
// entire path; blobPathForFormatVersion() must also be used. This is a separate call because this
// part of the filepath is constant, and blobPathForFormatVersion may need to be called multiple
// times with different storage.FormatVersion values.
func (dir *Dir) blobToBasePath(ref blobstore.BlobRef) (string, error) {
	return dir.refToDirPath(ref, dir.blobsdir)
}

// refToDirPath converts a blob reference to a filepath in the specified sub-directory.
func (dir *Dir) refToDirPath(ref blobstore.BlobRef, subDir string) (string, error) {
	if !ref.IsValid() {
		return "", blobstore.ErrInvalidBlobRef.New("")
	}

	r := make([]byte, 0, len(subDir)+1+PathEncoding.EncodedLen(len(ref.Namespace))+encodedKeyPathLen(ref.Key))
	r = append(r, []byte(subDir)...)
	r = append(r, filepath.Separator)
	r = base32AppendEncode(r, ref.Namespace)
	r = appendEncodedKeyPath(r, ref.Key)

	return unsafe.String(&r[0], len(r)), nil // using unsafe.String here to avoid an allocations.
}

func (dir *Dir) findBlobInTrash(ctx context.Context, ref blobstore.BlobRef) (dirTime time.Time, formatVer blobstore.FormatVersion, path string, err error) {
	defer mon.Task()(&ctx)(&err)

	err = dir.forEachTrashDayDir(ctx, ref.Namespace, func(dayDirTime time.Time) error {
		trashBasePath, err := dir.refToTrashPath(ref, dayDirTime)
		if err != nil {
			// something was wrong with our input; don't need to keep looking
			return err
		}
		for ver := MinFormatVersionSupportedInTrash; ver <= MaxFormatVersionSupported; ver++ {
			trashVerPath := blobPathForFormatVersion(trashBasePath, ver)
			_, err = os.Stat(trashVerPath)
			if err == nil {
				dirTime = dayDirTime
				path = trashVerPath
				formatVer = ver
				break
			}
		}
		return nil
	})
	if err != nil {
		return time.Time{}, 0, "", err
	}
	if path == "" {
		return time.Time{}, 0, "", os.ErrNotExist
	}
	return dirTime, formatVer, path, nil
}

// blobPathForFormatVersion adjusts a bare blob path (as might have been generated by a call to
// blobToBasePath()) to what it should be for the given storage format version.
func blobPathForFormatVersion(path string, formatVersion blobstore.FormatVersion) string {
	switch formatVersion {
	case FormatV0:
		return path + v0PieceFileSuffix
	case FormatV1:
		return path + v1PieceFileSuffix
	}
	return path + unknownPieceFileSuffix
}

// Commit commits the temporary file to permanent storage.
func (dir *Dir) Commit(ctx context.Context, file *os.File, sync bool, ref blobstore.BlobRef, formatVersion blobstore.FormatVersion) (err error) {
	defer mon.Task()(&ctx)(&err)
	var syncErr error
	if sync {
		syncErr = file.Sync()
	}

	closeErr := file.Close()

	if syncErr != nil || closeErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(syncErr, closeErr, removeErr)
	}

	path, err := dir.blobToBasePath(ref)
	if err != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(err, removeErr)
	}
	path = blobPathForFormatVersion(path, formatVersion)

	if file.Name() != path {
		mkdirErr := os.MkdirAll(filepath.Dir(path), dirPermission)
		if os.IsExist(mkdirErr) {
			mkdirErr = nil
		}
		if mkdirErr != nil {
			removeErr := os.Remove(file.Name())
			return errs.Combine(mkdirErr, removeErr)
		}

		renameErr := rename(file.Name(), path)
		if renameErr != nil {
			removeErr := os.Remove(file.Name())
			return errs.Combine(renameErr, removeErr)
		}
	}

	return nil
}

var monOpen = mon.Task()

// Open opens the file with the specified ref. It may need to check in more than one location in
// order to find the blob, if it was stored with an older version of the storage node software.
// In cases where the storage format version of a blob is already known, OpenWithStorageFormat()
// will generally be a better choice.
func (dir *Dir) Open(ctx context.Context, ref blobstore.BlobRef) (_ *os.File, _ blobstore.FormatVersion, err error) {
	defer monOpen(&ctx)(&err)

	path, err := dir.blobToBasePath(ref)
	if err != nil {
		return nil, FormatV0, err
	}
	for formatVer := MaxFormatVersionSupported; formatVer >= MinFormatVersionSupported; formatVer-- {
		vPath := blobPathForFormatVersion(path, formatVer)
		file, err := openFileReadOnly(vPath, blobPermission)
		if err == nil {
			return file, formatVer, nil
		}
		if !os.IsNotExist(err) {
			return nil, FormatV0, Error.New("unable to open %q: %v", vPath, err)
		}
	}
	return nil, FormatV0, os.ErrNotExist
}

// OpenWithStorageFormat opens an already-located blob file with a known storage format version,
// which avoids the potential need to search through multiple storage formats to find the blob.
func (dir *Dir) OpenWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (_ *os.File, err error) {
	defer mon.Task()(&ctx)(&err)
	path, err := dir.blobToBasePath(ref)
	if err != nil {
		return nil, err
	}
	vPath := blobPathForFormatVersion(path, formatVer)
	file, err := openFileReadOnly(vPath, blobPermission)
	if err == nil {
		return file, nil
	}
	if os.IsNotExist(err) {
		// we don't want to wrap something matching os.IsNotExist, because IsNotExist
		// does _not_ unwrap.
		return nil, err
	}
	return nil, Error.Wrap(err)
}

// Stat looks up disk metadata on the blob file. It may need to check in more than one location
// in order to find the blob, if it was stored with an older version of the storage node software.
// In cases where the storage format version of a blob is already known, StatWithStorageFormat()
// will generally be a better choice.
func (dir *Dir) Stat(ctx context.Context, ref blobstore.BlobRef) (_ blobstore.BlobInfo, err error) {
	// not monkit monitoring because of performance reasons

	path, err := dir.blobToBasePath(ref)
	if err != nil {
		return nil, err
	}
	for formatVer := MaxFormatVersionSupported; formatVer >= MinFormatVersionSupported; formatVer-- {
		vPath := blobPathForFormatVersion(path, formatVer)
		stat, err := os.Stat(vPath)
		if err == nil {
			return newBlobInfo(ref, vPath, stat, formatVer), nil
		}
		if !os.IsNotExist(err) {
			return nil, Error.New("unable to stat %q: %v", vPath, err)
		}
	}
	return nil, os.ErrNotExist
}

var monStatWithStorageFormat = mon.Task()

// StatWithStorageFormat looks up disk metadata on the blob file with the given storage format
// version. This avoids the need for checking for the file in multiple different storage format
// types.
func (dir *Dir) StatWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (_ blobstore.BlobInfo, err error) {
	defer monStatWithStorageFormat(&ctx)(&err)
	path, err := dir.blobToBasePath(ref)
	if err != nil {
		return nil, err
	}
	vPath := blobPathForFormatVersion(path, formatVer)
	stat, err := os.Stat(vPath)
	if err == nil {
		return newBlobInfo(ref, vPath, stat, formatVer), nil
	}
	if os.IsNotExist(err) {
		return nil, err
	}
	return nil, Error.New("unable to stat %q: %v", vPath, err)
}

var monTrash = mon.Task()

// Trash moves the blob specified by ref to the trash for every format version.
func (dir *Dir) Trash(ctx context.Context, ref blobstore.BlobRef, timestamp time.Time) (err error) {
	defer monTrash(&ctx)(&err)
	return dir.iterateStorageFormatVersions(ctx, ref, func(ctx context.Context, ref blobstore.BlobRef, formatVersion blobstore.FormatVersion) error {
		return dir.TrashWithStorageFormat(ctx, ref, formatVersion, timestamp)
	})
}

// TrashWithStorageFormat moves the blob specified by ref to the trash for the specified format version.
func (dir *Dir) TrashWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion, timestamp time.Time) (err error) {
	blobsBasePath, err := dir.blobToBasePath(ref)
	if err != nil {
		return err
	}

	blobsVerPath := blobPathForFormatVersion(blobsBasePath, formatVer)

	trashBasePath, err := dir.refToTrashPath(ref, timestamp)
	if err != nil {
		return err
	}

	trashVerPath := blobPathForFormatVersion(trashBasePath, formatVer)

	// move to trash
	err = rename(blobsVerPath, trashVerPath)
	if errors.Is(err, fs.ErrNotExist) {
		// ensure that trash dir is not what's missing
		err = os.MkdirAll(filepath.Dir(trashVerPath), dirPermission)
		if err != nil && !os.IsExist(err) {
			return err
		}

		// try rename once again
		err = rename(blobsVerPath, trashVerPath)
		if !errors.Is(err, fs.ErrNotExist) {
			dir.log.Debug("blob not found; will not trash", zap.String("blob_path", blobsVerPath), zap.Error(err))
			return err
		}

		// no blob at that path; either it has a different storage format
		// version or there was a concurrent call. (This function is expected
		// by callers to return a nil error in the case of concurrent calls.)
		return nil
	}
	return err
}

// RestoreTrash moves every blob in the trash folder back into blobsdir.
func (dir *Dir) RestoreTrash(ctx context.Context, namespace []byte) (keysRestored [][]byte, err error) {
	var errorsEncountered errs.Group
	err = dir.walkNamespaceInTrash(ctx, namespace, func(info blobstore.BlobInfo, dirTime time.Time) error {
		blobsBasePath, err := dir.blobToBasePath(info.BlobRef())
		if err != nil {
			errorsEncountered.Add(err)
			return nil
		}

		blobsVerPath := blobPathForFormatVersion(blobsBasePath, info.StorageFormatVersion())

		trashBasePath, err := dir.refToTrashPath(info.BlobRef(), dirTime)
		if err != nil {
			errorsEncountered.Add(err)
			return nil
		}

		trashVerPath := blobPathForFormatVersion(trashBasePath, info.StorageFormatVersion())

		// ensure the dirs exist for blobs path
		err = os.MkdirAll(filepath.Dir(blobsVerPath), dirPermission)
		if err != nil && !os.IsExist(err) {
			errorsEncountered.Add(err)
			return nil
		}

		// move back to blobsdir
		err = rename(trashVerPath, blobsVerPath)
		if os.IsNotExist(err) {
			// no blob at that path; either it has a different storage format
			// version or there was a concurrent call. (This function is expected
			// by callers to return a nil error in the case of concurrent calls.)
			return nil
		}
		if err != nil {
			errorsEncountered.Add(err)
			return nil
		}

		keysRestored = append(keysRestored, info.BlobRef().Key)
		return nil
	})
	errorsEncountered.Add(err)
	return keysRestored, errorsEncountered.Err()
}

// TryRestoreTrashBlob attempts to restore a blob from the trash if it exists.
// It returns nil if the blob was restored, or an error if the blob was not
// in the trash or could not be restored.
func (dir *Dir) TryRestoreTrashBlob(ctx context.Context, ref blobstore.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, formatVer, blobPathInTrash, err := dir.findBlobInTrash(ctx, ref)
	if err != nil {
		return err
	}

	blobsBasePath, err := dir.blobToBasePath(ref)
	if err != nil {
		return err
	}

	// ensure the dirs exist for blobs path in main storage
	blobsVerPath := blobPathForFormatVersion(blobsBasePath, formatVer)
	err = os.MkdirAll(filepath.Dir(blobsVerPath), dirPermission)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	// move back to main storage
	return rename(blobPathInTrash, blobsVerPath)
}

// EmptyTrashWithoutStat is like EmptyTrash, but without calculating the newly available space.
func (dir *Dir) EmptyTrashWithoutStat(ctx context.Context, namespace []byte, trashedBefore time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = dir.forEachTrashDayDir(ctx, namespace, func(dirTime time.Time) error {
		// add 24h since blobs in there might have been moved in as late as 23:59:59.999
		if !dirTime.Add(24 * time.Hour).After(trashedBefore) {
			trashPath := dir.trashPath(namespace, dirTime)

			subdirNames, err := readAllDirNames(trashPath)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}

			for _, keyPrefix := range subdirNames {
				if len(keyPrefix) != 2 {
					// just an invalid subdir; could be garbage of many kinds. probably
					// don't need to pass on this error
					continue
				}

				trashPrefix := filepath.Join(trashPath, keyPrefix)
				err := dir.deleteAllTrashFiles(ctx, trashPrefix)
				if errors.Is(err, context.Canceled) {
					return nil
				}
				if err != nil {
					dir.log.Warn("Couldn't delete trash directory", zap.String("dir", trashPrefix), zap.Error(err))
				}

				// if directory is empty, remove it
				_ = dir.removeDirButLogIfNotExist(trashPrefix)

			}

			_ = dir.removeDirButLogIfNotExist(trashPath)

		}
		return nil
	})
	return nil
}

var deletedFiles = mon.Counter("trash_deleted_files")

func (dir *Dir) deleteAllTrashFiles(ctx context.Context, prefixDir string) error {
	openDir, err := os.Open(prefixDir)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, openDir.Close()) }()
	for {
		// check for context done both before and after our readdir() call
		if err := ctx.Err(); err != nil {
			return err
		}
		names, err := openDir.Readdirnames(nameBatchSize)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if os.IsNotExist(err) || len(names) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, name := range names {
			err := dir.removeButLogIfNotExist(filepath.Join(prefixDir, name))
			if err != nil {
				return errs.Wrap(err)
			}
			deletedFiles.Inc(1)
			// also check for context done between every walkFunc callback.
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
}

// EmptyTrash iterates through the toplevel trash directories for the given
// namespace and recursively deletes any of them more than 24h older than
// trashedBefore.
func (dir *Dir) EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (bytesEmptied int64, deletedKeys [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	var errorsEncountered errs.Group
	err = dir.forEachTrashDayDir(ctx, namespace, func(dirTime time.Time) error {
		// add 24h since blobs in there might have been moved in as late as 23:59:59.999
		if !dirTime.Add(24 * time.Hour).After(trashedBefore) {
			emptied, keys, err := dir.deleteTrashDayDir(ctx, namespace, dirTime)
			bytesEmptied += emptied
			deletedKeys = append(deletedKeys, keys...)
			errorsEncountered.Add(err)
		}
		return nil
	})
	errorsEncountered.Add(err)
	return bytesEmptied, deletedKeys, errorsEncountered.Err()
}

// DeleteTrashNamespace deletes an entire namespace under the trash dir.
func (dir *Dir) DeleteTrashNamespace(ctx context.Context, namespace []byte) (err error) {
	mon.Task()(&ctx)(&err)
	var errorsEncountered errs.Group
	err = dir.forEachTrashDayDir(ctx, namespace, func(dirTime time.Time) error {
		_, _, err := dir.deleteTrashDayDir(ctx, namespace, dirTime)
		errorsEncountered.Add(err)
		return nil
	})
	errorsEncountered.Add(err)
	namespaceEncoded := PathEncoding.EncodeToString(namespace)
	namespaceTrashDir := filepath.Join(dir.trashdir, namespaceEncoded)
	err = dir.removeDirButLogIfNotExist(namespaceTrashDir)
	errorsEncountered.Add(err)
	return errorsEncountered.Err()
}

// walkNamespaceInTrash executes walkFunc for each blob stored in the trash under the given
// namespace. If walkFunc returns a non-nil error, walkNamespaceInTrash will stop iterating and
// return the error immediately. The ctx parameter is intended specifically to allow canceling
// iteration early.
func (dir *Dir) walkNamespaceInTrash(ctx context.Context, namespace []byte, f func(info blobstore.BlobInfo, dirTime time.Time) error) error {
	return dir.forEachTrashDayDir(ctx, namespace, func(dirTime time.Time) error {
		return dir.walkTrashDayDir(ctx, namespace, dirTime, func(info blobstore.BlobInfo) error {
			return f(info, dirTime)
		})
	})
}

func (dir *Dir) forEachTrashDayDir(ctx context.Context, namespace []byte, f func(dirTime time.Time) error) error {
	dirTimes, err := dir.listTrashDayDirs(ctx, namespace)
	if err != nil {
		return err
	}
	for _, dirTime := range dirTimes {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = f(dirTime)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dir *Dir) walkTrashDayDir(ctx context.Context, namespace []byte, dirTime time.Time, f func(info blobstore.BlobInfo) error) (err error) {
	trashPath := dir.trashPath(namespace, dirTime)
	return dir.walkNamespaceUnderPath(ctx, namespace, trashPath, nil, f)
}

func (dir *Dir) listTrashDayDirs(ctx context.Context, namespace []byte) (dirTimes []time.Time, err error) {
	namespaceEncoded := PathEncoding.EncodeToString(namespace)
	namespaceTrashDir := filepath.Join(dir.trashdir, namespaceEncoded)
	openDir, err := os.Open(namespaceTrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			dir.log.Debug("directory not found", zap.String("dir", namespaceTrashDir))
			// job accomplished: there are no day dirs in this namespace!
			return nil, nil
		}
		return nil, err
	}
	defer func() { err = errs.Combine(err, openDir.Close()) }()
	for {
		// check for context done both before and after our readdir() call
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		subdirNames, err := openDir.Readdirnames(nameBatchSize)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return dirTimes, nil
			}
			return nil, err
		}
		if len(subdirNames) == 0 {
			return dirTimes, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, subdirName := range subdirNames {
			subdirTime, err := time.Parse("2006-01-02", subdirName)
			if err != nil {
				// just an invalid subdir; could be garbage of many kinds. probably
				// don't need to pass on this error
				continue
			}
			dirTimes = append(dirTimes, subdirTime)
		}
	}
}

func (dir *Dir) removeDirButLogIfNotExist(dirPath string) error {
	err := rmDir(dirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			dir.log.Debug("directory not found; nothing to remove", zap.String("dir", dirPath))
			return nil
		}
		return err
	}
	return nil
}

func (dir *Dir) removeButLogIfNotExist(pathToRemove string) error {
	err := os.Remove(pathToRemove)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			dir.log.Debug("file not found; nothing to remove", zap.String("path", pathToRemove))
			return nil
		}
		return err
	}
	return nil
}

func (dir *Dir) deleteTrashDayDir(ctx context.Context, namespace []byte, dirTime time.Time) (bytesEmptied int64, deletedKeys [][]byte, err error) {
	var errorsEncountered errs.Group
	err = dir.walkTrashDayDir(ctx, namespace, dirTime, func(info blobstore.BlobInfo) error {
		thisBlobInfo, ok := info.(*blobInfo)
		if !ok {
			// if this happens, it's time to extend the code to handle the other type
			errorsEncountered.Add(Error.New("%+v [unexpected type %T]: %w", info, info, err))
			return nil
		}
		fileInfo, err := info.Stat(ctx)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			if errors.Is(err, ErrIsDir) {
				return nil
			}
			errorsEncountered.Add(Error.New("%s: %w", thisBlobInfo.path, err))
			return nil
		}
		err = dir.removeButLogIfNotExist(thisBlobInfo.path)
		if err != nil {
			errorsEncountered.Add(err)
			return nil
		}
		bytesEmptied += fileInfo.Size()
		deletedKeys = append(deletedKeys, info.BlobRef().Key)
		return nil
	})
	if err != nil {
		errorsEncountered.Add(err)
		return bytesEmptied, deletedKeys, errorsEncountered.Err()
	}
	// Finish by attempting to remove the directory structure for this timestamp
	// (this will fail if any files are left undeleted inside). This works like
	// `rmdir "trash/$namespace/$timestamp"/??; rmdir "trash/$namespace/$timestamp"`.
	trashDayDir := dir.trashPath(namespace, dirTime)
	dirEntries, err := os.ReadDir(trashDayDir)
	if err != nil {
		errorsEncountered.Add(Error.New("list %s: %w", trashDayDir, err))
		return bytesEmptied, deletedKeys, errorsEncountered.Err()
	}
	for _, entry := range dirEntries {
		if entry.IsDir() && len(entry.Name()) == 2 {
			err = dir.removeDirButLogIfNotExist(filepath.Join(trashDayDir, entry.Name()))
			errorsEncountered.Add(err)
		}
	}
	err = dir.removeDirButLogIfNotExist(trashDayDir)
	errorsEncountered.Add(err)
	return bytesEmptied, deletedKeys, errorsEncountered.Err()
}

var monIterateStorageFormatVersions = mon.Task()

// iterateStorageFormatVersions executes f for all storage format versions,
// starting with the oldest format version. It is more likely, in the general
// case, that we will find the blob with the newest format version instead,
// but if we iterate backward here then we run the risk of a race condition:
// the blob might have existed with _SomeOldVer before the call, and could
// then have been updated atomically with _MaxVer concurrently while we were
// iterating. If we iterate _forwards_, this race should not occur because it
// is assumed that blobs are never rewritten with an _older_ storage format
// version.
//
// f will be executed for every storage format version regardless of the
// result, and will aggregate errors into a single returned error.
func (dir *Dir) iterateStorageFormatVersions(ctx context.Context, ref blobstore.BlobRef, f func(ctx context.Context, ref blobstore.BlobRef, i blobstore.FormatVersion) error) (err error) {
	defer monIterateStorageFormatVersions(&ctx)(&err)
	var combinedErrors errs.Group
	for i := MinFormatVersionSupported; i <= MaxFormatVersionSupported; i++ {
		combinedErrors.Add(f(ctx, ref, i))
	}
	return combinedErrors.Err()
}

// Delete deletes blobs with the specified ref (in all supported storage formats).
//
// It doesn't return an error if the blob is not found for any reason.
func (dir *Dir) Delete(ctx context.Context, ref blobstore.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	return dir.iterateStorageFormatVersions(ctx, ref, dir.DeleteWithStorageFormat)
}

// DeleteWithStorageFormat deletes the blob with the specified ref for one
// specific format version.
//
// It doesn't return an error if the blob isn't found for any reason.
func (dir *Dir) DeleteWithStorageFormat(ctx context.Context, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (err error) {
	// not monkit monitoring because of performance reasons

	return dir.deleteWithStorageFormatInPath(ctx, dir.blobsdir, ref, formatVer)
}

// DeleteNamespace deletes blobs folder for a specific namespace.
func (dir *Dir) DeleteNamespace(ctx context.Context, ref []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return dir.deleteNamespace(ctx, dir.blobsdir, ref)
}

func (dir *Dir) deleteWithStorageFormatInPath(ctx context.Context, path string, ref blobstore.BlobRef, formatVer blobstore.FormatVersion) (err error) {
	// not monkit monitoring because of performance reasons

	pathBase, err := dir.refToDirPath(ref, path)
	if err != nil {
		return err
	}

	verPath := blobPathForFormatVersion(pathBase, formatVer)

	// try removing the file
	return dir.removeButLogIfNotExist(verPath)
}

// deleteNamespace deletes folder with everything inside.
func (dir *Dir) deleteNamespace(ctx context.Context, path string, ref []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	namespace := PathEncoding.EncodeToString(ref)
	folderPath := filepath.Join(path, namespace)

	err = os.RemoveAll(folderPath)
	return err
}

const nameBatchSize = 1024

// ListNamespaces finds all known namespace IDs in use in local storage. They are not
// guaranteed to contain any blobs.
func (dir *Dir) ListNamespaces(ctx context.Context) (ids [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	return dir.listNamespacesInPath(ctx, dir.blobsdir)
}

// listNamespacesInTrash lists all known the namespace IDs in use in the trash. They are
// not guaranteed to contain any blobs, or to correspond to namespaces in main storage.
func (dir *Dir) listNamespacesInTrash(ctx context.Context) (ids [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	return dir.listNamespacesInPath(ctx, dir.trashdir)
}

func (dir *Dir) listNamespacesInPath(ctx context.Context, path string) (ids [][]byte, err error) {
	defer mon.Task()(&ctx)(&err)
	openDir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, openDir.Close()) }()
	for {
		dirNames, err := openDir.Readdirnames(nameBatchSize)
		if err != nil {
			if errors.Is(err, io.EOF) || os.IsNotExist(err) {
				return ids, nil
			}
			return ids, err
		}
		if len(dirNames) == 0 {
			return ids, nil
		}
		for _, name := range dirNames {
			namespace, err := PathEncoding.DecodeString(name)
			if err != nil {
				// just an invalid directory entry, and not a namespace. probably
				// don't need to pass on this error
				continue
			}
			ids = append(ids, namespace)
		}
	}
}

// WalkNamespace executes walkFunc for each locally stored blob, stored with storage format V1 or
// greater, in the given namespace. If walkFunc returns a non-nil error, WalkNamespace will stop
// iterating and return the error immediately. The ctx parameter is intended specifically to allow
// canceling iteration early.
func (dir *Dir) WalkNamespace(ctx context.Context, namespace []byte, skipPrefixFn blobstore.SkipPrefixFn, walkFunc func(blobstore.BlobInfo) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	return dir.walkNamespaceInPath(ctx, namespace, dir.blobsdir, skipPrefixFn, walkFunc)
}

func (dir *Dir) walkNamespaceInPath(ctx context.Context, namespace []byte, path string, skipPrefixFn blobstore.SkipPrefixFn, walkFunc func(blobstore.BlobInfo) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	namespaceDir := PathEncoding.EncodeToString(namespace)
	nsDir := filepath.Join(path, namespaceDir)
	return dir.walkNamespaceUnderPath(ctx, namespace, nsDir, skipPrefixFn, walkFunc)
}

func (dir *Dir) walkNamespaceUnderPath(ctx context.Context, namespace []byte, nsDir string, skipPrefixFn blobstore.SkipPrefixFn, walkFunc func(blobstore.BlobInfo) error) (err error) {
	subdirNames, err := readAllDirNames(nsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			dir.log.Debug("directory not found", zap.String("dir", nsDir))
			// job accomplished: there are no blobs in this namespace!
			return nil
		}
		return err
	}

	dir.log.Debug("number of subdirs", zap.Int("count", len(subdirNames)))

	// sort the dir names, so we can start from the startPrefix
	sortPrefixes(subdirNames)

	for _, keyPrefix := range subdirNames {
		if len(keyPrefix) != 2 {
			// just an invalid subdir; could be garbage of many kinds. probably
			// don't need to pass on this error
			continue
		}

		if skipPrefixFn != nil && skipPrefixFn(keyPrefix) {
			continue
		}
		err := walkNamespaceWithPrefix(ctx, namespace, nsDir, keyPrefix, walkFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func readAllDirNames(dir string) (subDirNames []string, err error) {
	openDir, err := os.Open(dir)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, openDir.Close())
	}()

	for {
		names, err := openDir.Readdirnames(nameBatchSize)
		if err != nil {
			if errors.Is(err, io.EOF) || os.IsNotExist(err) {
				break
			}
			return subDirNames, err
		}
		if len(names) == 0 {
			return subDirNames, nil
		}

		subDirNames = append(subDirNames, names...)
	}

	return subDirNames, nil
}

// migrateTrashToPerDayDirs migrates a trash directory that is _not_ using per-day directories
// to a trash directory that _does_ use per-day directories. This is accomplished by shunting
// everything in the trash into the directory for the current day.
//
// This will result in some things staying in the trash for longer than they otherwise would
// have, but it is likely that operators will welcome the improvement in performance anyway.
//
// In short, this moves:
//
//	trash/$namespace/?? -> trash/$namespace/$day/??
//
// Or, in shell syntax, we are doing:
//
//	mv trash/$namespace trash/$namespace-$day && \
//	mkdir trash/$namespace && \
//	mv trash/$namespace-$day trash/$namespace/$day
//
// This approach does the minimum number of filesystem changes to perform the migration.
func (dir *Dir) migrateTrashToPerDayDirs(now time.Time) (err error) {
	defer mon.Task()(nil)(&err)

	namespaces, err := dir.listNamespacesInTrash(context.Background())
	for _, ns := range namespaces {
		nsEncoded := PathEncoding.EncodeToString(ns)
		todayDirName := now.Format("2006-01-02")
		nsPath := filepath.Join(dir.trashdir, nsEncoded)
		tempTodayDirPath := filepath.Join(dir.trashdir, nsEncoded+"-"+todayDirName)
		dir.log.Info("migrating trash namespace to use per-day directories", zap.String("namespace", nsEncoded))
		err = os.Rename(nsPath, tempTodayDirPath)
		if err != nil {
			return err
		}
		err = os.Mkdir(nsPath, dirPermission)
		if err != nil {
			return err
		}
		err = os.Rename(tempTodayDirPath, filepath.Join(nsPath, todayDirName))
		if err != nil {
			return err
		}
		dir.log.Info("trash namespace migration complete", zap.String("namespace", nsEncoded))
	}
	return nil
}

// decodeBlobInfo expects keyPrefix, keyDir and blobFilename all to be clean.
func decodeBlobInfo(namespace []byte, keyPrefix, keyDir, blobFileName string) (info *blobInfo, ok bool) {
	encodedKey := keyPrefix + blobFileName
	formatVer := FormatV0
	if strings.HasSuffix(blobFileName, v1PieceFileSuffix) {
		formatVer = FormatV1
		encodedKey = encodedKey[0 : len(encodedKey)-len(v1PieceFileSuffix)]
	}
	// in case we prepended '1' chars because the key was too short (1 is an invalid char in base32)
	encodedKey = strings.TrimLeft(encodedKey, "1")
	key, err := PathEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, false
	}
	ref := blobstore.BlobRef{
		Namespace: namespace,
		Key:       key,
	}
	return newBlobInfo(ref, keyDir+string(filepath.Separator)+blobFileName, nil, formatVer), true
}

func walkNamespaceWithPrefix(ctx context.Context, namespace []byte, nsDir, keyPrefix string, walkFunc func(blobstore.BlobInfo) error) (err error) {
	keyDir := filepath.Join(nsDir, keyPrefix)
	openDir, err := os.Open(keyDir)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, openDir.Close()) }()
	for {
		// check for context done both before and after our readdir() call
		if err := ctx.Err(); err != nil {
			return err
		}
		names, err := openDir.Readdirnames(nameBatchSize)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if os.IsNotExist(err) || len(names) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, name := range names {
			blobInfo, ok := decodeBlobInfo(namespace, keyPrefix, keyDir, name)
			if !ok {
				continue
			}
			err = walkFunc(blobInfo)
			if err != nil {
				return err
			}
			// also check for context done between every walkFunc callback.
			if err := ctx.Err(); err != nil {
				return err
			}
		}
	}
}

// Info returns information about the current state of the dir.
func (dir *Dir) Info(ctx context.Context) (blobstore.DiskInfo, error) {
	return dir.info.AvailableSpace(ctx)
}

type blobInfo struct {
	ref           blobstore.BlobRef
	path          string
	fileInfo      os.FileInfo
	formatVersion blobstore.FormatVersion
}

func newBlobInfo(ref blobstore.BlobRef, path string, fileInfo os.FileInfo, formatVer blobstore.FormatVersion) *blobInfo {
	return &blobInfo{
		ref:           ref,
		path:          path,
		fileInfo:      fileInfo,
		formatVersion: formatVer,
	}
}

func (info *blobInfo) BlobRef() blobstore.BlobRef {
	return info.ref
}

func (info *blobInfo) StorageFormatVersion() blobstore.FormatVersion {
	return info.formatVersion
}

func (info *blobInfo) Stat(ctx context.Context) (blobstore.FileInfo, error) {
	if info.fileInfo == nil {
		fileInfo, err := os.Lstat(info.path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, err
			}
			if isLowLevelCorruptionError(err) {
				return nil, &CorruptDataError{path: info.path, error: err}
			}
			return nil, err
		}
		if fileInfo.Mode().IsDir() {
			return fileInfo, ErrIsDir
		}
		info.fileInfo = fileInfo
	}
	return info.fileInfo, nil
}

func (info *blobInfo) FullPath(ctx context.Context) (string, error) {
	return info.path, nil
}

// CorruptDataError represents a filesystem or disk error which indicates data corruption.
//
// We use a custom error type here so that we can add explanatory information and wrap the original
// error at the same time.
type CorruptDataError struct {
	path  string
	error error
}

// Unwrap unwraps the error.
func (cde CorruptDataError) Unwrap() error {
	return cde.error
}

// Path returns the path at which the error was encountered.
func (cde CorruptDataError) Path() string {
	return cde.path
}

// Error returns an error string describing the condition.
func (cde CorruptDataError) Error() string {
	return fmt.Sprintf("unrecoverable error accessing data on the storage file system (path=%v; error=%v). This is most likely due to disk bad sectors or a corrupted file system. Check your disk for bad sectors and integrity", cde.path, cde.error)
}

// sortPrefixes sorts the given prefixes in a way that it puts a-z before 0-9.
func sortPrefixes(prefixes []string) {
	slices.SortStableFunc(prefixes, func(a, b string) int {
		if a[0] == b[0] {
			if isDigit(a[1]) && isLetter(b[1]) {
				return 1 // a (numeric) comes after b (alphabet)
			}
			if isLetter(a[1]) && isDigit(b[1]) {
				return -1 // a (alphabet) comes before b (numeric)
			}
		}
		if isDigit(a[0]) && isLetter(b[0]) {
			return 1 // a (numeric) comes after b (alphabet)
		}
		if isLetter(a[0]) && isDigit(b[0]) {
			return -1 // a (alphabet) comes before b (numeric)
		}
		// Default behavior: compare strings lexicographically
		return strings.Compare(a, b)
	})
}

func isDigit(r byte) bool {
	return '0' <= r && r <= '9'
}

func isLetter(r byte) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

func trashSubdirLength(namespace []byte) int {
	return 1 + PathEncoding.EncodedLen(len(namespace)) + 1 + 10
}

// appendTrashSubdir appends the trash directory for the given namespace and timestamp.
func appendTrashSubdir(r []byte, namespace []byte, forTime time.Time) []byte {
	r = append(r, filepath.Separator)
	r = base32AppendEncode(r, namespace)

	r = append(r, filepath.Separator)
	r = forTime.UTC().AppendFormat(r, "2006-01-02")

	return r
}

func encodedKeyPathLen(key []byte) int {
	n := PathEncoding.EncodedLen(len(key))
	if n < 3 {
		n += 2
	}
	return 2 + n
}

func appendEncodedKeyPath(r, key []byte) []byte {
	r = append(r, filepath.Separator)

	// The following implements creating subdirecotries,
	// but without creating intermediate strings
	//   key := PathEncoding.EncodeToString(ref.Key)
	//   if len(key) <= 3 { key = "11' + key }
	//   r = r + "/" + key[:2] + "/" + key[2:]

	at := len(r)
	r = append(r, 0) // sentinel
	if PathEncoding.EncodedLen(len(key)) < 3 {
		// ensure we always have enough characters to split [:2] and [2:]
		r = append(r, '1', '1')
	}
	r = base32AppendEncode(r, key)

	r[at] = r[at+1]
	r[at+1] = r[at+2]
	r[at+2] = filepath.Separator

	return r
}

func base32AppendEncode(dst, src []byte) []byte {
	// This duplicates PathEncoding.AppendEncode, which is available in Go 1.22+.
	n := PathEncoding.EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	PathEncoding.Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}
