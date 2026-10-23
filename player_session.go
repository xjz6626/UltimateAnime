package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type playerSession struct {
	cancel context.CancelFunc
	done   chan struct{}
	id     string
}

type mpvEvent struct {
	Event     string          `json:"event"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
	Error     string          `json:"error"`
	RequestID int             `json:"request_id"`
}

type playbackTracker struct {
	progress      PlaybackProgress
	lastPosition  float64
	lastSample    time.Time
	paused        bool
	seeking       bool
	speed         float64
	path          string
	pathConfirmed bool
}

// Positive movement is counted only when compatible with elapsed playback time.
// Seeking and replaying the same span do not inflate viewed coverage.
func (t *playbackTracker) consume(event mpvEvent, now time.Time) (flush, ended, normalEnd bool) {
	if event.Event == "end-file" {
		return true, true, event.Reason == "eof"
	}
	if event.Event == "shutdown" {
		return true, true, false
	}
	if event.Event == "seek" {
		t.lastSample = time.Time{}
		return false, false, false
	}
	if event.Event != "property-change" || string(event.Data) == "null" {
		return false, false, false
	}
	switch event.Name {
	case "path":
		var path string
		if json.Unmarshal(event.Data, &path) != nil {
			return
		}
		if filepath.Clean(path) != filepath.Clean(t.path) {
			return true, true, false
		}
		t.pathConfirmed = true
	case "pause":
		var value bool
		if json.Unmarshal(event.Data, &value) != nil {
			return
		}
		t.paused = value
		t.lastSample = time.Time{}
		flush = value
	case "seeking":
		if json.Unmarshal(event.Data, &t.seeking) == nil && t.seeking {
			t.lastSample = time.Time{}
		}
	case "speed":
		var value float64
		if json.Unmarshal(event.Data, &value) == nil && value > 0 && validSeconds(value) {
			t.speed = value
			t.lastSample = time.Time{}
		}
	case "duration":
		var value float64
		if json.Unmarshal(event.Data, &value) == nil && value > 0 && validSeconds(value) {
			t.progress.Duration = value
		}
	case "time-pos":
		var value float64
		if !t.pathConfirmed || json.Unmarshal(event.Data, &value) != nil || !validSeconds(value) {
			return
		}
		if !t.lastSample.IsZero() && !t.paused && !t.seeking {
			delta := value - t.lastPosition
			elapsed := now.Sub(t.lastSample).Seconds()
			if delta > 0 && delta <= elapsed*max(t.speed, 1)+0.75 && elapsed <= 15 {
				t.progress.Ranges = mergeViewed(t.progress.Ranges, t.lastPosition, value)
			}
		}
		t.progress.Position = value
		t.progress.UpdatedAt = now
		t.lastPosition = value
		t.lastSample = now
	}
	return
}

func (a *App) launchLocalPlayer(subjectID int, episode float64, path string, restart bool) string {
	a.playerMu.Lock()
	defer a.playerMu.Unlock()
	return a.launchLocalPlayerLocked(subjectID, episode, path, restart)
}

func (a *App) launchLocalPlayerLocked(subjectID int, episode float64, path string, restart bool) string {
	if a.taskContext().Err() != nil {
		return "Error: 应用正在退出"
	}
	if a.playerSessions == nil {
		a.playerSessions = map[string]*playerSession{}
	}
	key := downloadKey(subjectID, episode)
	if a.playerSessions[key] != nil {
		if restart {
			return "Error: 请先关闭这一集的播放器，再从头播放"
		}
		return "Success"
	}
	a.cancelNextLocked()
	settings := a.configMgr.Snapshot().Player
	userArgs, err := parsePlayerArgs(settings.MPVArgs)
	if err != nil {
		return "Error: " + err.Error()
	}
	// IPC and start position are managed; avoid silently overriding user options.
	for _, arg := range userArgs {
		if strings.HasPrefix(arg, "--input-ipc-") || strings.HasPrefix(arg, "--playlist") {
			return "Error: 播放进度模式不支持自定义 IPC 或播放列表参数"
		}
	}
	version, err := mediaVersion(path)
	if err != nil {
		return "Error: " + err.Error()
	}
	list, err := a.readLibrary()
	if err != nil {
		return "Error: " + err.Error()
	}
	progress := PlaybackProgress{MediaVersion: version}
	for _, item := range list {
		if item.SubjectID == subjectID {
			old := item.Playback[fmt.Sprint(episode)]
			if !restart && old.MediaVersion == version && !old.Completed {
				progress = old
			}
			break
		}
	}
	progress.Completed = false
	progress.Hidden = false
	directory, err := os.MkdirTemp("", "ua-mpv-")
	if err != nil {
		return "Error: " + err.Error()
	}
	address := playerIPCAddress(directory)
	progress.SessionID = filepath.Base(directory)
	position := progress.Position
	if progress.Duration > 0 {
		position = min(position, max(0, progress.Duration-0.1))
	}
	args := append([]string{"--force-window"}, userArgs...)
	args = append(args, a.localDanmakuArgs(path)...)
	args = append(args, "--input-ipc-server="+address, "--pause=yes", "--start="+fmt.Sprintf("%.3f", position), "--", path)
	cmd := exec.Command(a.playerExecutable(settings.MPVPath), args...)
	cmd.Dir = a.dataDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.RemoveAll(directory)
		return fmt.Sprintf("Error: 启动 MPV 失败: %v", err)
	}
	// Install the new session before accepting progress from it.
	result := a.updateLibraryItem(subjectID, func(item *FollowedItem) error {
		if a.absoluteMediaPath(item.LocalFiles[fmt.Sprint(episode)]) != path {
			return fmt.Errorf("视频记录已更新，请重新播放")
		}
		if item.Playback == nil {
			item.Playback = map[string]PlaybackProgress{}
		}
		item.Playback[fmt.Sprint(episode)] = progress
		return nil
	})
	if result != "Success" {
		cmd.Process.Kill()
		cmd.Wait()
		os.RemoveAll(directory)
		return result
	}
	ctx, cancel := context.WithCancel(a.taskContext())
	session := &playerSession{cancel: cancel, done: make(chan struct{}), id: progress.SessionID}
	a.playerSessions[key] = session
	processDone := make(chan struct{})
	go func() { _ = cmd.Wait(); close(processDone) }()
	go func() {
		defer close(session.done)
		defer cancel()
		defer os.RemoveAll(directory)
		normal, completed, err := a.trackLocalPlayer(ctx, processDone, address, path, subjectID, episode, progress)
		if err != nil {
			// This process was started paused; don't leave a stalled player behind.
			_ = cmd.Process.Kill()
			a.Log("ERROR", err.Error())
			if ctx.Err() == nil {
				a.emit("playback-error", err.Error())
			}
		}
		a.playerMu.Lock()
		if a.playerSessions[key] == session {
			delete(a.playerSessions, key)
		}
		if normal && completed && ctx.Err() == nil && len(a.playerSessions) == 0 {
			a.scheduleNextLocked(subjectID, episode)
		}
		a.playerMu.Unlock()
	}()
	return "Success"
}

func (a *App) trackLocalPlayer(ctx context.Context, processDone <-chan struct{}, address, path string, subjectID int, episode float64, progress PlaybackProgress) (bool, bool, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var conn io.ReadWriteCloser
	var err error
	for {
		conn, err = dialPlayerIPC(connectCtx, address)
		if err == nil {
			break
		}
		select {
		case <-connectCtx.Done():
			return false, false, fmt.Errorf("无法连接 MPV 播放进度接口: %w", err)
		case <-processDone:
			a.Log("WARN", "MPV 在连接播放进度接口前退出")
			return false, false, fmt.Errorf("MPV 在连接播放进度接口前退出，请检查播放器路径与参数")
		case <-time.After(30 * time.Millisecond):
		}
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { conn.Close() })
	defer stopClose()
	writer := json.NewEncoder(conn)
	for id, name := range []string{"path", "duration", "time-pos", "pause", "seeking", "speed"} {
		if err := writer.Encode(map[string]any{"command": []any{"observe_property", id + 1, name}, "request_id": id + 1}); err != nil {
			a.Log("ERROR", "订阅播放进度失败: "+err.Error())
			return false, false, fmt.Errorf("订阅播放进度失败: %w", err)
		}
	}
	if err := writer.Encode(map[string]any{"command": []any{"set_property", "pause", false}, "request_id": 7}); err != nil {
		return false, false, fmt.Errorf("恢复播放失败: %w", err)
	}
	normal, completed := a.consumePlayerStream(ctx, conn, subjectID, episode, path, progress)
	return normal, completed, nil
}

func (a *App) consumePlayerStream(ctx context.Context, stream io.Reader, subjectID int, episode float64, path string, progress PlaybackProgress) (bool, bool) {
	tracker := playbackTracker{progress: progress, path: path, speed: 1}
	lastSave := time.Now()
	savedPosition := progress.Position
	normal, completed := false, false
	save := func(mark bool) bool {
		if !tracker.pathConfirmed || tracker.progress.UpdatedAt.IsZero() {
			return false
		}
		lastSave = time.Now()
		tracker.progress.Completed = mark
		if err := a.savePlayback(subjectID, episode, path, tracker.progress, mark); err != nil {
			a.Log("ERROR", "保存播放进度失败: "+err.Error())
			a.emit("playback-error", "保存播放进度失败: "+err.Error())
			return false
		}
		savedPosition = tracker.progress.Position
		return true
	}
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var event mpvEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.RequestID > 0 && event.Error != "" && event.Error != "success" {
			a.Log("WARN", "MPV 命令失败: "+event.Error)
		}
		flush, ended, eof := tracker.consume(event, time.Now())
		if ended {
			normal = eof
			completed = eof && tracker.pathConfirmed && tracker.progress.Duration > 0 && watchedDuration(tracker.progress) >= tracker.progress.Duration*0.9 && tracker.progress.Position >= tracker.progress.Duration*0.9
			if !save(completed) {
				completed = false
			}
			return normal, completed
		}
		if flush || time.Since(lastSave) >= 5*time.Second && math.Abs(tracker.progress.Position-savedPosition) >= 0.1 {
			save(false)
		}
		if ctx.Err() != nil {
			break
		}
	}
	save(false)
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		a.Log("WARN", "MPV 进度连接已断开: "+err.Error())
	}
	return false, false
}
