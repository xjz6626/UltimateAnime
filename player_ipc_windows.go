//go:build windows

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

func playerIPCAddress(directory string) string { return `\\.\pipe\` + filepath.Base(directory) }
func dialPlayerIPC(ctx context.Context, address string) (io.ReadWriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.OpenFile(address, os.O_RDWR, 0600)
}
