package main

import (
	"context"
	"fmt"

	"UltimateAnime/pkg/pikpak"
)

type downloadRun struct {
	cancel context.CancelFunc
	done   chan struct{}
	stop   string // protected by downloadMu
}

func (a *App) PauseDownload(id string) string  { return a.stopDownload(id, "paused") }
func (a *App) CancelDownload(id string) string { return a.stopDownload(id, "cancelled") }

func (a *App) stopDownload(id, target string) string {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	if err := a.loadDownloadsLocked(); err != nil {
		return "Error: " + err.Error()
	}
	task, ok := a.downloads[id]
	if !ok {
		return "Error: 未找到任务"
	}
	if task.State == "completed" {
		return "Error: 视频已完成，请在剧集菜单管理本地视频"
	}
	if task.DanmakuState == "preparing" {
		task.DanmakuState = "interrupted"
	}
	if run := a.downloadRuns[id]; run != nil {
		if run.stop == "cancelled" || run.stop == target {
			return "Success"
		}
		task.State = "pausing"
		if target == "cancelled" {
			task.State = "cancelling"
		}
		if err := a.saveDownloadLocked(task); err != nil {
			return "Error: " + err.Error()
		}
		run.stop = target
		run.cancel()
	} else {
		if target == "cancelled" {
			if err := pikpak.DiscardPartial(task.Path); err != nil {
				return "Error: 清理下载缓存失败: " + err.Error()
			}
			if !task.LocalReady {
				task.Progress = 0
			}
		}
		task.State, task.Error = target, ""
		if err := a.saveDownloadLocked(task); err != nil {
			return "Error: " + err.Error()
		}
	}
	if cancel := a.downloadDanmaku[id]; cancel != nil {
		cancel()
		delete(a.downloadDanmaku, id)
	}
	return "Success"
}

func (a *App) finishDownloadRun(id string, run *downloadRun) {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	if a.downloadRuns[id] != run {
		return
	}
	defer close(run.done)
	defer delete(a.downloadRuns, id)
	task := a.downloads[id]
	if task.State == "completed" {
		return
	}
	if run.stop == "" {
		return
	}
	task.State, task.Error = run.stop, ""
	if run.stop == "cancelled" {
		if err := pikpak.DiscardPartial(task.Path); err != nil {
			task.State = "failed"
			task.Error = fmt.Sprintf("取消后清理缓存失败，可再次取消重试: %v", err)
		} else if !task.LocalReady {
			task.Progress = 0
		}
	}
	if err := a.saveDownloadLocked(task); err != nil {
		task.Error = "保存停止状态失败: " + err.Error()
		a.downloads[id] = task
		a.emit("download-task", task)
		a.Log("ERROR", task.Error)
	}
}
