package main

import (
	"context"
	"time"
)

func (a *App) shutdown(_ context.Context) {
	if a.workCancel != nil {
		a.workCancel()
	}
	var pending []<-chan struct{}
	a.playerMu.Lock()
	a.cancelNextLocked()
	for _, session := range a.playerSessions {
		session.cancel()
		pending = append(pending, session.done)
	}
	a.playerMu.Unlock()
	a.downloadMu.Lock()
	for _, run := range a.downloadRuns {
		run.cancel()
		pending = append(pending, run.done)
	}
	for _, cancel := range a.downloadDanmaku {
		cancel()
	}
	a.downloadMu.Unlock()
	// Allow IPC readers to flush and download workers to persist checkpoints.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for _, done := range pending {
		select {
		case <-done:
		case <-deadline.C:
			return
		}
	}
}
