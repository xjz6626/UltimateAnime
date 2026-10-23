package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"UltimateAnime/pkg/pikpak"
	"UltimateAnime/pkg/storage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// A task owns its account and cloud IDs even after the active account changes.
type DownloadTask struct {
	ID           string    `json:"id"`
	SubjectID    int       `json:"subject_id"`
	Episode      float64   `json:"ep_sort"`
	Name         string    `json:"name"`
	Magnet       string    `json:"magnet"`
	Account      string    `json:"account"`
	CloudID      string    `json:"cloud_id"`
	CloudFailed  bool      `json:"cloud_failed,omitempty"`
	FileID       string    `json:"file_id"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	Directory    string    `json:"directory"`
	Path         string    `json:"path"`
	State        string    `json:"state"`
	Progress     float64   `json:"progress"`
	Error        string    `json:"error"`
	Retries      int       `json:"retries"`
	LocalReady   bool      `json:"local_ready"`
	DanmakuState string    `json:"danmaku_state"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func activeDownload(state string) bool {
	return state == "queued" || state == "cloud_pending" || state == "downloading" || state == "pausing" || state == "cancelling"
}
func downloadKey(id int, episode float64) string { return fmt.Sprintf("%d:%g", id, episode) }
func (a *App) taskContext() context.Context {
	if a.workCtx != nil {
		return a.workCtx
	}
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
	a.webEvents.publish(name, data)
}

func (a *App) loadDownloadsLocked() error {
	if a.downloads != nil {
		return nil
	}
	tasks := map[string]DownloadTask{}
	data, err := os.ReadFile(filepath.Join(a.dataDir(), "downloads.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal(data, &tasks); err != nil {
			return fmt.Errorf("下载任务文件损坏，原文件已保留: %w", err)
		}
		if tasks == nil {
			return fmt.Errorf("下载任务文件不能为 null")
		}
	}
	for id, task := range tasks {
		if task.DanmakuState == "preparing" {
			task.DanmakuState = "interrupted"
		}
		if activeDownload(task.State) {
			task.State = "interrupted"
			task.Error = "应用退出中断；可继续已有云端任务"
		}
		tasks[id] = task
	}
	a.downloads = tasks
	a.downloadSlots = make(chan struct{}, 2)
	a.downloadRuns = map[string]*downloadRun{}
	return nil
}

func (a *App) GetDownloadTasks() ([]DownloadTask, error) {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	if err := a.loadDownloadsLocked(); err != nil {
		return nil, err
	}
	tasks := make([]DownloadTask, 0, len(a.downloads))
	for _, task := range a.downloads {
		tasks = append(tasks, task)
	}
	slices.SortFunc(tasks, func(a, b DownloadTask) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return tasks, nil
}

func (a *App) saveDownloadLocked(task DownloadTask) error {
	previous, existed := a.downloads[task.ID]
	task.UpdatedAt = time.Now()
	a.downloads[task.ID] = task
	if err := storage.WriteJSON(filepath.Join(a.dataDir(), "downloads.json"), a.downloads); err != nil {
		if existed {
			a.downloads[task.ID] = previous
		} else {
			delete(a.downloads, task.ID)
		}
		return err
	}
	a.emit("download-task", task)
	return nil
}

func (a *App) updateDownload(task *DownloadTask) error {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	// The independent danmaku worker may have finished since this snapshot.
	if latest, ok := a.downloads[task.ID]; ok {
		if latest.Retries != task.Retries {
			return fmt.Errorf("任务已被新的一次运行替代")
		}
		task.DanmakuState = latest.DanmakuState
		if latest.State == "pausing" || latest.State == "cancelling" {
			task.State = latest.State
		}
	}
	return a.saveDownloadLocked(*task)
}

func (a *App) downloadDirectory(subjectID int) (string, string, error) {
	list, err := a.readLibrary()
	if err != nil {
		return "", "", err
	}
	name := ""
	for _, item := range list {
		if item.SubjectID == subjectID {
			name = item.NameCN
			if name == "" {
				name = item.Name
			}
			break
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("请先追番")
	}
	root := strings.TrimSpace(a.configMgr.Snapshot().LocalStorage.AnimeDir)
	if root == "" {
		root = "Downloads"
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(a.dataDir(), root)
	}
	directory := filepath.Join(root, fmt.Sprintf("%s_%d", sanitizePathSegment(name), subjectID))
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", "", err
	}
	probe, err := os.CreateTemp(directory, ".write-check-*")
	if err != nil {
		return "", "", fmt.Errorf("下载目录不可写: %w", err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return directory, name, nil
}

func (a *App) DownloadEpisode(subjectID int, episode float64, magnet string) string {
	if subjectID <= 0 || episode <= 0 || math.IsNaN(episode) || math.IsInf(episode, 0) || !strings.HasPrefix(strings.ToLower(magnet), "magnet:?xt=urn:btih:") {
		return "Error: 无效的番剧、集数或磁力链接"
	}
	if a.configMgr == nil {
		return "Error: PikPak 配置未初始化"
	}
	if err := a.configMgr.LoadError(); err != nil {
		return "Error: 配置加载失败: " + err.Error()
	}
	settings := a.configMgr.Snapshot().GlobalSettings
	if a.currentPikPakClient() == nil && (len(settings.PikPakUsers) == 0 || settings.PikPakPassword == "") {
		return "Error: 请先在桌面设置中配置 PikPak 账号和密码"
	}
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()
	if err := a.loadDownloadsLocked(); err != nil {
		return "Error: " + err.Error()
	}
	key := downloadKey(subjectID, episode)
	if task, ok := a.downloads[key]; ok && (activeDownload(task.State) || a.downloadRuns[key] != nil) {
		return "Existing"
	}
	var task DownloadTask
	old, exists := a.downloads[key]
	if exists && old.Magnet == magnet {
		if old.State == "completed" {
			if info, err := os.Stat(old.Path); err == nil && info.Mode().IsRegular() && info.Size() == old.FileSize {
				return "Completed"
			}
		}
		task = old
		if old.State == "completed" {
			task.LocalReady = false
		}
		task.State, task.Error = "queued", ""
		task.Retries++
		if task.CloudFailed {
			task.CloudID, task.FileID, task.Account = "", "", ""
			task.CloudFailed, task.LocalReady = false, false
		}
	} else {
		directory, name, err := a.downloadDirectory(subjectID)
		if err != nil {
			return "Error: " + err.Error()
		}
		task = DownloadTask{ID: key, SubjectID: subjectID, Episode: episode, Name: name, Magnet: magnet, Directory: directory, State: "queued"}
		if exists {
			if err := pikpak.DiscardPartial(old.Path); err != nil {
				return "Error: 清理旧资源缓存失败: " + err.Error()
			}
			task.Retries = old.Retries + 1
		}
	}
	if err := a.saveDownloadLocked(task); err != nil {
		return "Error: 保存下载任务失败: " + err.Error()
	}
	ctx, cancel := context.WithCancel(a.taskContext())
	run := &downloadRun{cancel: cancel, done: make(chan struct{})}
	a.downloadRuns[task.ID] = run
	go a.runDownloadTask(ctx, task, run)
	return "Started"
}

func (a *App) RetryDownload(id string) string {
	a.downloadMu.Lock()
	if err := a.loadDownloadsLocked(); err != nil {
		a.downloadMu.Unlock()
		return "Error: " + err.Error()
	}
	task, ok := a.downloads[id]
	a.downloadMu.Unlock()
	if !ok {
		return "Error: 未找到任务"
	}
	return a.DownloadEpisode(task.SubjectID, task.Episode, task.Magnet)
}

var errCloudTaskFailed = errors.New("云端下载失败")

func (a *App) waitOfflineTask(ctx context.Context, client *pikpak.PikPakClient, task pikpak.OfflineTask) (pikpak.OfflineTask, error) {
	if task.Phase == "PHASE_TYPE_COMPLETE" && task.FileID != "" {
		return task, nil
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return task, ctx.Err()
		case <-ticker.C:
		}
		tasks, err := client.OfflineList(true)
		if err != nil {
			a.Log("WARN", "云端任务暂时无法查询，将继续跟踪: "+err.Error())
			continue
		}
		for _, next := range tasks {
			if next.ID == task.ID {
				if next.Phase == "PHASE_TYPE_ERROR" {
					return next, fmt.Errorf("%w: %s", errCloudTaskFailed, next.Message)
				}
				if next.Phase == "PHASE_TYPE_COMPLETE" && next.FileID != "" {
					return next, nil
				}
			}
		}
	}
}

func (a *App) taskClient(task DownloadTask) (*pikpak.PikPakClient, error) {
	if task.Account == "" {
		if err := a.loginForDownload(); err != nil {
			return nil, err
		}
		return a.currentPikPakClient(), nil
	}
	if a.isAccountBlocked(task.Account) {
		return nil, fmt.Errorf("任务所属账号已停用或暂不可用，请恢复账号后重试")
	}
	if client := a.currentPikPakClient(); client != nil && client.Username == task.Account {
		return client, nil
	}
	cfg := a.configMgr.Snapshot().GlobalSettings
	if !slices.Contains(cfg.PikPakUsers, task.Account) {
		return nil, fmt.Errorf("任务所属账号已移除，请恢复账号后重试")
	}
	client := pikpak.NewPikPakClient(task.Account, cfg.PikPakPassword, cfg.Proxy)
	if err := client.Login(); err != nil {
		return nil, err
	}
	return client, nil
}

func (a *App) runDownloadTask(ctx context.Context, task DownloadTask, run *downloadRun) {
	defer a.finishDownloadRun(task.ID, run)
	defer run.cancel()
	select {
	case a.downloadSlots <- struct{}{}:
		defer func() { <-a.downloadSlots }()
	case <-ctx.Done():
		task.State = "interrupted"
		task.Error = "应用退出，任务已中断"
		_ = a.updateDownload(&task)
		return
	}
	fail := func(err error) {
		task.State = "failed"
		if ctx.Err() != nil {
			task.State = "interrupted"
		}
		task.Error = err.Error()
		if saveErr := a.updateDownload(&task); saveErr != nil {
			// Keep failure visible in the live snapshot even if the disk is unavailable.
			task.Error += "; 保存任务失败: " + saveErr.Error()
			a.downloadMu.Lock()
			a.downloads[task.ID] = task
			a.downloadMu.Unlock()
			a.emit("download-task", task)
		}
		if ctx.Err() != nil {
			return
		}
		a.Log("ERROR", "下载失败: "+task.Error)
		a.emit("download-error", map[string]any{"subject_id": task.SubjectID, "ep_sort": task.Episode, "error": task.Error})
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return
	}
	if task.LocalReady {
		if info, err := os.Stat(task.Path); err != nil || !info.Mode().IsRegular() || info.Size() != task.FileSize {
			task.LocalReady = false
		}
	}
	if !task.LocalReady {
		client, err := a.taskClient(task)
		if err != nil {
			fail(err)
			return
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		if task.FileID == "" {
			cloud := pikpak.OfflineTask{ID: task.CloudID}
			if task.CloudID == "" {
				created, owner, err := a.addOfflineTaskWithContext(ctx, client, task.Magnet, len(a.configMgr.Snapshot().GlobalSettings.PikPakUsers), a.nextPikPakClient)
				if err != nil {
					fail(err)
					return
				}
				cloud, client = *created, owner
				task.Account, task.CloudID = client.Username, cloud.ID
			}
			task.State = "cloud_pending"
			// Preserve instant-upload file IDs before any network/file operations.
			if cloud.Phase == "PHASE_TYPE_COMPLETE" {
				task.FileID, task.FileName = cloud.FileID, cloud.FileName
			}
			if err := a.updateDownload(&task); err != nil {
				fail(err)
				return
			}
			completed, err := a.waitOfflineTask(ctx, client, cloud)
			if err != nil {
				task.CloudFailed = errors.Is(err, errCloudTaskFailed)
				fail(err)
				return
			}
			task.FileID, task.FileName = completed.FileID, completed.FileName
			if task.FileName == "" {
				task.FileName = completed.Name
			}
			if err := a.updateDownload(&task); err != nil {
				fail(err)
				return
			}
		}
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}
		fileID, name, size, _, err := a.resolvePikPakFile(client, task.FileID, task.FileName, task.Episode)
		if err != nil {
			fail(err)
			return
		}
		task.FileID, task.FileName, task.FileSize = fileID, name, size
		if err := os.MkdirAll(task.Directory, 0755); err != nil {
			fail(err)
			return
		}
		if task.Path == "" {
			task.Path = filepath.Join(task.Directory, fmt.Sprintf("%g_%s", task.Episode, sanitizePathSegment(name)))
		}
		task.State = "downloading"
		if err := a.updateDownload(&task); err != nil {
			fail(err)
			return
		}
		a.startTaskDanmaku(task, false)
		last := time.Now()
		var progressErr error
		err = client.DownloadFileResumable(ctx, task.FileID, task.Path, task.FileSize, 16, func(current, total int64) {
			if total > 0 && (time.Since(last) >= time.Second || current == total || task.Progress == 0) {
				task.Progress = float64(current) * 100 / float64(total)
				last = time.Now()
				if err := a.updateDownload(&task); err != nil {
					progressErr = err
				}
			}
		})
		if err != nil {
			fail(err)
			return
		}
		task.LocalReady = true
		if err := a.updateDownload(&task); err != nil {
			fail(err)
			return
		}
		if progressErr != nil {
			a.Log("WARN", "下载过程中进度保存暂时失败，完成状态已恢复保存")
		}
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return
	}
	if err := a.markEpisodeDownloaded(task.SubjectID, task.Episode, task.Path); err != nil {
		fail(err)
		return
	}
	task.State, task.Progress, task.Error = "completed", 100, ""
	if err := a.updateDownload(&task); err != nil {
		fail(err)
		return
	}
	if task.State != "completed" {
		return
	}
	a.emit("download-complete", map[string]any{"subject_id": task.SubjectID, "ep_sort": task.Episode, "path": task.Path})
}

func (a *App) startTaskDanmaku(task DownloadTask, refresh bool) {
	a.downloadMu.Lock()
	current, ok := a.downloads[task.ID]
	if !ok || current.Path != task.Path || current.Retries != task.Retries || !refresh && current.State != "downloading" {
		a.downloadMu.Unlock()
		return
	}
	if a.downloadDanmaku == nil {
		a.downloadDanmaku = map[string]context.CancelFunc{}
	}
	if cancel := a.downloadDanmaku[task.ID]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(a.taskContext())
	a.downloadDanmaku[task.ID] = cancel
	current.DanmakuState = "preparing"
	err := a.saveDownloadLocked(current)
	a.downloadMu.Unlock()
	if err != nil {
		a.Log("WARN", "保存弹幕状态失败: "+err.Error())
	}
	go func() {
		defer cancel()
		a.prepareDanmaku(ctx, task.SubjectID, task.Episode, task.Path, refresh)
		if ctx.Err() != nil {
			return
		}
		state := "unavailable"
		if info, err := os.Stat(danmakuPath(task.Path)); err == nil && info.Size() > 0 {
			state = "ready"
		}
		a.downloadMu.Lock()
		defer a.downloadMu.Unlock()
		current, ok := a.downloads[task.ID]
		if ctx.Err() != nil || !ok || current.Path != task.Path || current.Retries != task.Retries {
			return
		}
		delete(a.downloadDanmaku, task.ID)
		current.DanmakuState = state
		if err := a.saveDownloadLocked(current); err != nil {
			a.Log("WARN", "保存弹幕状态失败: "+err.Error())
		}
	}()
}
