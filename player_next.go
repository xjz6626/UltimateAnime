package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"
)

type NextPlayback struct {
	SubjectID int       `json:"subject_id"`
	Episode   float64   `json:"ep_sort"`
	StartsAt  time.Time `json:"starts_at"`
}

func (a *App) cancelNextLocked() {
	hadPending := a.nextPlayback != nil
	if a.nextPlaybackCancel != nil {
		a.nextPlaybackCancel()
		a.nextPlaybackCancel = nil
	}
	a.nextPlayback = nil
	if hadPending {
		a.emit("playback-next", nil)
	}
}
func (a *App) CancelNextPlayback() {
	a.playerMu.Lock()
	a.cancelNextLocked()
	a.playerMu.Unlock()
}
func (a *App) GetNextPlayback() *NextPlayback {
	a.playerMu.Lock()
	defer a.playerMu.Unlock()
	if a.nextPlayback == nil {
		return nil
	}
	copy := *a.nextPlayback
	return &copy
}
func (a *App) scheduleNextLocked(subjectID int, episode float64) {
	if a.configMgr == nil || !a.configMgr.Snapshot().Player.AutoNext || episode != math.Trunc(episode) {
		return
	}
	list, err := a.readLibrary()
	if err != nil {
		return
	}
	for _, item := range list {
		if item.SubjectID == subjectID {
			path := a.absoluteMediaPath(item.LocalFiles[fmt.Sprint(episode+1)])
			if path == "" {
				return
			}
			if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
				return
			}
			a.cancelNextLocked()
			pending := &NextPlayback{SubjectID: subjectID, Episode: episode + 1, StartsAt: time.Now().Add(8 * time.Second)}
			ctx, cancel := context.WithCancel(a.taskContext())
			a.nextPlaybackCancel = cancel
			a.nextPlayback = pending
			a.emit("playback-next", pending)
			go func() {
				defer cancel()
				timer := time.NewTimer(8 * time.Second)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
				if result := a.startNextPlayback(pending); result != "Success" {
					a.Log("ERROR", "播放下一集失败: "+result)
					a.emit("playback-error", "播放下一集失败: "+result)
				}
			}()
			return
		}
	}
}

// Cancellation, manual playback, and countdown expiry share the launch lock.
func (a *App) startNextPlayback(pending *NextPlayback) string {
	a.playerMu.Lock()
	defer a.playerMu.Unlock()
	if a.nextPlayback != pending {
		return "Success"
	}
	a.cancelNextLocked()
	if a.taskContext().Err() != nil || !a.configMgr.Snapshot().Player.AutoNext || len(a.playerSessions) > 0 {
		return "Success"
	}
	list, err := a.readLibrary()
	if err != nil {
		return "Error: " + err.Error()
	}
	for _, item := range list {
		if item.SubjectID == pending.SubjectID {
			path := a.absoluteMediaPath(item.LocalFiles[fmt.Sprint(pending.Episode)])
			if path != "" {
				return a.launchLocalPlayerLocked(pending.SubjectID, pending.Episode, path, false)
			}
		}
	}
	return "Error: 下一集的本地视频已不存在"
}
