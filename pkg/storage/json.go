// Package storage writes user data without truncating the last valid copy.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if !json.Valid(old) {
			return fmt.Errorf("%s 已损坏；请先修复文件或从 .bak 恢复，原文件未覆盖", path)
		}
		if err := writeAtomic(path+".bak", old); err != nil {
			return fmt.Errorf("备份失败: %w", err)
		}
	}
	return writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	// Directory sync is best effort on filesystems/platforms that support it.
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
