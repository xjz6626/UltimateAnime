package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
	"UltimateAnime/pkg/pikpak"
)

func TestQueuedDownloadPauseAndCancelDoNotStartNetwork(t *testing.T) {
	a := libraryApp(t)
	a.pikpakClient = pikpak.NewPikPakClient("owner", "password", "")
	a.FollowLocal(bangumi.Subject{ID: 42, Name: "Test"})
	if _, err := a.GetDownloadTasks(); err != nil {
		t.Fatal(err)
	}
	// Hold both slots; stopping a queued job must not wait for a slot.
	a.downloadSlots <- struct{}{}
	a.downloadSlots <- struct{}{}
	if result := a.DownloadEpisode(42, 1, "magnet:?xt=urn:btih:example"); result != "Started" {
		t.Fatal(result)
	}
	if result := a.PauseDownload("42:1"); result != "Success" {
		t.Fatal(result)
	}
	task := waitDownload(t, a, "paused")
	if task.CloudID != "" {
		t.Fatal("paused queue created cloud task")
	}
	path := filepath.Join(a.dataDir(), "episode.mkv")
	for _, suffix := range []string{"", ".ua.part", ".ua.resume.json", ".ua.resume.json.bak"} {
		if err := os.WriteFile(path+suffix, []byte("keep final video"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a.downloadMu.Lock()
	task.Path = path
	task.Progress = 50
	a.downloads[task.ID] = task
	a.downloadMu.Unlock()
	if result := a.CancelDownload(task.ID); result != "Success" {
		t.Fatal(result)
	}
	task = waitDownload(t, a, "cancelled")
	if task.Progress != 0 {
		t.Fatal("cancel retained transfer progress")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancel removed final video", err)
	}
	if _, err := os.Stat(path + ".ua.part"); !os.IsNotExist(err) {
		t.Fatal("cancel retained partial")
	}
	if result := a.RetryDownload(task.ID); result != "Started" {
		t.Fatal(result)
	}
	if result := a.CancelDownload(task.ID); result != "Success" {
		t.Fatal(result)
	}
	waitDownload(t, a, "cancelled")
}

func TestStopDoesNotAllowRetryUntilWorkersExit(t *testing.T) {
	a := libraryApp(t)
	a.pikpakClient = pikpak.NewPikPakClient("owner", "password", "")
	a.GetDownloadTasks()
	ctx, cancel := context.WithCancel(context.Background())
	run := &downloadRun{cancel: cancel, done: make(chan struct{})}
	task := DownloadTask{ID: "42:1", SubjectID: 42, Episode: 1, State: "downloading", Magnet: "magnet:?xt=urn:btih:example"}
	a.downloadRuns[task.ID] = run
	a.downloads[task.ID] = task
	if result := a.PauseDownload(task.ID); result != "Success" {
		t.Fatal(result)
	}
	if ctx.Err() == nil {
		t.Fatal("worker was not cancelled")
	}
	if result := a.RetryDownload(task.ID); result != "Existing" {
		t.Fatal("overlapping worker allowed", result)
	}
	if result := a.CancelDownload(task.ID); result != "Success" {
		t.Fatal(result)
	}
	// A late progress callback must not erase the user's stop intent.
	if err := a.updateDownload(&task); err != nil {
		t.Fatal(err)
	}
	if task.State != "cancelling" {
		t.Fatal(task.State)
	}
	a.finishDownloadRun(task.ID, run)
	waitDownload(t, a, "cancelled")
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("worker not released")
	}
}

func TestShutdownWaitsForPlaybackFlushAndDownloadExit(t *testing.T) {
	a := libraryApp(t)
	a.workCtx, a.workCancel = context.WithCancel(context.Background())
	playerCtx, playerCancel := context.WithCancel(a.workCtx)
	runCtx, runCancel := context.WithCancel(a.workCtx)
	playerDone, runDone := make(chan struct{}), make(chan struct{})
	a.playerSessions = map[string]*playerSession{"42:1": {cancel: playerCancel, done: playerDone}}
	a.downloadRuns = map[string]*downloadRun{"42:1": {cancel: runCancel, done: runDone}}
	go func() { <-playerCtx.Done(); close(playerDone) }()
	go func() { <-runCtx.Done(); close(runDone) }()
	a.shutdown(context.Background())
	select {
	case <-playerDone:
	default:
		t.Fatal("shutdown did not wait for progress")
	}
	select {
	case <-runDone:
	default:
		t.Fatal("shutdown did not wait for download")
	}
}

func TestCancellationPreventsCloudRecoveryActions(t *testing.T) {
	a := libraryApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := pikpak.NewPikPakClient("owner", "password", "")
	calls := 0
	client.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return pikpakJSONResponse(req, 200, `{"error_code":8,"error":"file_space_not_enough"}`), nil
	}))
	_, _, err := a.addOfflineTaskWithContext(ctx, client, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		t.Fatal("cancelled task switched account")
		return nil, nil
	})
	if err != context.Canceled || calls != 1 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}
