package pikpak

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"UltimateAnime/pkg/storage"
)

const downloadBlockSize int64 = 4 * 1024 * 1024

type resumeManifest struct {
	Version   int              `json:"version"`
	Identity  string           `json:"identity"`
	Size      int64            `json:"size"`
	BlockSize int64            `json:"block_size"`
	Completed map[int64]string `json:"completed"`
}

type resumeFile struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	manifest resumeManifest
}

func partialPaths(destination string) (string, string) {
	return destination + ".ua.part", destination + ".ua.resume.json"
}

// DiscardPartial removes only this application's incomplete transfer artifacts.
// Call only after all workers have stopped using the destination.
func DiscardPartial(destination string) error {
	if destination == "" {
		return nil
	}
	part, manifest := partialPaths(destination)
	for _, path := range []string{part, manifest, manifest + ".bak"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func openResumeFile(ctx context.Context, destination, identity string, size int64) (*resumeFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	part, manifestPath := partialPaths(destination)
	manifest := resumeManifest{Version: 1, Identity: identity, Size: size, BlockSize: downloadBlockSize, Completed: map[int64]string{}}
	data, err := os.ReadFile(manifestPath)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if exists {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("断点记录损坏，请取消任务清理缓存后重新下载: %w", err)
		}
		if manifest.Version != 1 || manifest.Identity != identity || manifest.Size != size || manifest.BlockSize != downloadBlockSize || manifest.Completed == nil {
			return nil, fmt.Errorf("断点记录与当前资源不符，请取消任务清理缓存后重新下载")
		}
	}
	file, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("下载缓存不是普通文件")
	}
	if !exists || info.Size() != size {
		manifest.Completed = map[int64]string{}
		if err := file.Truncate(size); err != nil {
			return nil, err
		}
	}
	// Persisted bits alone are insufficient: verify completed blocks after a crash
	// or external modification. A bad block is fetched again, not trusted.
	for index, checksum := range manifest.Completed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if index < 0 || index >= (size+downloadBlockSize-1)/downloadBlockSize {
			delete(manifest.Completed, index)
			continue
		}
		actual, err := blockChecksum(file, index, size)
		if err != nil || actual != checksum {
			delete(manifest.Completed, index)
		}
	}
	if err := storage.WriteJSON(manifestPath, manifest); err != nil {
		return nil, err
	}
	ok = true
	return &resumeFile{file: file, path: manifestPath, manifest: manifest}, nil
}

func blockChecksum(file *os.File, index, size int64) (string, error) {
	start := index * downloadBlockSize
	length := min(downloadBlockSize, size-start)
	sum := sha256.New()
	n, err := io.Copy(sum, io.NewSectionReader(file, start, length))
	if err != nil {
		return "", err
	}
	if n != length {
		return "", io.ErrUnexpectedEOF
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func (r *resumeFile) complete(index int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Data reaches disk before the durable completion record.
	if err := r.file.Sync(); err != nil {
		return err
	}
	checksum, err := blockChecksum(r.file, index, r.manifest.Size)
	if err != nil {
		return err
	}
	r.manifest.Completed[index] = checksum
	if err := storage.WriteJSON(r.path, r.manifest); err != nil {
		delete(r.manifest.Completed, index)
		return err
	}
	return nil
}

func (r *resumeFile) completedBytes() int64 {
	var size int64
	for index := range r.manifest.Completed {
		size += min(downloadBlockSize, r.manifest.Size-index*downloadBlockSize)
	}
	return size
}
