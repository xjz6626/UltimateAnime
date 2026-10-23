package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
	"UltimateAnime/pkg/pikpak"
	"UltimateAnime/pkg/storage"
)

func waitDownload(t *testing.T, a *App, state string) DownloadTask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tasks, err := a.GetDownloadTasks()
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) == 1 && tasks[0].State == state {
			return tasks[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	tasks, _ := a.GetDownloadTasks()
	t.Fatalf("expected %s: %+v", state, tasks)
	return DownloadTask{}
}

func TestDownloadsDeduplicateAndCompleteBeforeDanmaku(t *testing.T) {
	a := libraryApp(t)
	a.configMgr.Data.LocalStorage.AnimeDir = "自定义 有空格"
	a.FollowLocal(bangumi.Subject{ID: 42, Name: "Test"})
	a.SaveEpisodeDanmakuSource(42, 1, DanmakuChoice{Kind: "skip"})
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "episode.mkv", time.Time{}, strings.NewReader("video"))
	}))
	defer media.Close()
	client := pikpak.NewPikPakClient("first", "password", "")
	var creates atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	client.Client.SetTransport(pikpakRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/v1/shield/captcha/init":
			return pikpakJSONResponse(r, 200, `{"captcha_token":"captcha"}`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/drive/v1/files":
			if creates.Add(1) == 1 {
				close(started)
				<-release
			}
			return pikpakJSONResponse(r, 200, `{"file":{"id":"video","name":"episode.mkv"}}`), nil
		default:
			return pikpakJSONResponse(r, 200, fmt.Sprintf(`{"id":"video","name":"episode.mkv","size":"5","kind":"drive#file","web_content_link":%q}`, media.URL)), nil
		}
	}))
	a.pikpakClient = client
	// A slow danmaku preparation must not delay the video completion event.
	a.danmakuMu.Lock()
	var once sync.Once
	unlock := func() { once.Do(func() { a.danmakuMu.Unlock() }) }
	defer unlock()
	if result := a.DownloadEpisode(42, 1, "magnet:?xt=urn:btih:example"); result != "Started" {
		t.Fatal(result)
	}
	<-started
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if result := a.DownloadEpisode(42, 1, "magnet:?xt=urn:btih:other"); result != "Existing" {
				t.Error(result)
			}
		}()
	}
	group.Wait()
	a.UnfollowLocal(42)
	close(release)
	task := waitDownload(t, a, "completed")
	if creates.Load() != 1 {
		t.Fatalf("created %d cloud tasks", creates.Load())
	}
	if task.DanmakuState != "preparing" {
		t.Fatalf("video waited for comments: %+v", task)
	}
	if !strings.HasPrefix(task.Path, filepath.Join(a.dataDir(), "自定义 有空格", "Test_42")) {
		t.Fatal(task.Path)
	}
	data, err := os.ReadFile(task.Path)
	if err != nil || string(data) != "video" {
		t.Fatalf("video=%s err=%v", data, err)
	}
	list, _ := a.readLibrary()
	if !list[0].Archived || list[0].LocalFiles["1"] != task.Path {
		t.Fatalf("archived download lost: %+v", list)
	}
	unlock()
	// Wait for the sidecar worker before temporary-directory cleanup.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		tasks, _ := a.GetDownloadTasks()
		if tasks[0].DanmakuState != "preparing" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("danmaku worker did not finish")
}

func TestRestartRetainsCloudTaskAndManualRetryDoesNotCreateAgain(t *testing.T) {
	a := libraryApp(t)
	a.configMgr.Data.Danmaku.Enabled = false
	a.FollowLocal(bangumi.Subject{ID: 42, Name: "Test"})
	directory, _, err := a.downloadDirectory(42)
	if err != nil {
		t.Fatal(err)
	}
	saved := DownloadTask{ID: "42:1", SubjectID: 42, Episode: 1, Name: "Test", Magnet: "magnet:?xt=urn:btih:example", Account: "owner", CloudID: "existing", Directory: directory, State: "cloud_pending"}
	if err := storage.WriteJSON(filepath.Join(a.dataDir(), "downloads.json"), map[string]DownloadTask{saved.ID: saved}); err != nil {
		t.Fatal(err)
	}
	tasks, err := a.GetDownloadTasks()
	if err != nil || tasks[0].State != "interrupted" || tasks[0].CloudID != "existing" {
		t.Fatalf("snapshot=%+v err=%v", tasks, err)
	}
	client := pikpak.NewPikPakClient("owner", "password", "")
	var posts atomic.Int32
	client.Client.SetTransport(pikpakRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		if r.URL.Path == "/drive/v1/tasks" {
			return pikpakJSONResponse(r, 200, `{"tasks":[{"id":"existing","phase":"PHASE_TYPE_COMPLETE","file_id":"video"}]}`), nil
		}
		// A deterministic failure after polling proves retry used the old task.
		return pikpakJSONResponse(r, 404, `{}`), nil
	}))
	a.pikpakClient = client
	// A changed, unusable new root must not prevent continuing the old directory.
	blockedRoot := filepath.Join(a.dataDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	a.configMgr.Data.LocalStorage.AnimeDir = blockedRoot
	if result := a.RetryDownload(saved.ID); result != "Started" {
		t.Fatal(result)
	}
	task := waitDownload(t, a, "failed")
	if posts.Load() != 0 || task.CloudID != "existing" || task.FileID != "video" {
		t.Fatalf("retry lost identity: %+v posts=%d", task, posts.Load())
	}
}

func TestCloudPollingWaitsAndCancelsWithoutNewTask(t *testing.T) {
	a := libraryApp(t)
	client := pikpak.NewPikPakClient("owner", "password", "")
	client.Client.SetTransport(pikpakRoundTripFunc(func(r *http.Request) (*http.Response, error) { return pikpakJSONResponse(r, 200, `{"tasks":[]}`), nil }))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.waitOfflineTask(ctx, client, pikpak.OfflineTask{ID: "existing"}); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestDisabledCurrentAccountCannotCreateNewTask(t *testing.T) {
	a := libraryApp(t)
	client := pikpak.NewPikPakClient("owner", "password", "")
	a.pikpakClient = client
	a.configMgr.Data.GlobalSettings.PikPakUsers = []string{"owner"}
	a.configMgr.Data.GlobalSettings.PikPakPassword = "password"
	if result := a.SetAccountBlockStatus("owner", true); result != "Success" {
		t.Fatal(result)
	}
	client.Client.SetTransport(pikpakRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Error("disabled account received API request")
		return pikpakJSONResponse(r, 500, `{}`), nil
	}))
	if _, _, err := a.addOfflineTaskWithRecovery(client, "magnet:test", 1, nil); err == nil {
		t.Fatal("disabled account accepted task")
	}
	if a.GetBlockedAccounts()["owner"] != "已停用" {
		t.Fatal("wrong account state")
	}
}
