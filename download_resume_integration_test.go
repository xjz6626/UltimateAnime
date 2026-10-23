package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
	"UltimateAnime/pkg/config"
	"UltimateAnime/pkg/pikpak"
)

func TestDownloadPauseRestartAndResumeIntegration(t *testing.T) {
	const blockSize = 4 * 1024 * 1024
	data := bytes.Repeat([]byte("x"), blockSize+17)
	var firstBlockRequests, cloudCreates atomic.Int32
	var finish atomic.Bool
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			t.Error(err)
			http.Error(w, "range required", 400)
			return
		}
		if start == 0 {
			firstBlockRequests.Add(1)
		}
		if start == blockSize && !finish.Load() {
			<-r.Context().Done()
			return
		}
		http.ServeContent(w, r, "episode.mkv", time.Time{}, bytes.NewReader(data))
	}))
	newClient := func() *pikpak.PikPakClient {
		client := pikpak.NewPikPakClient("owner", "password", "")
		client.Client.SetTransport(pikpakRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.URL.Path == "/v1/shield/captcha/init":
				return pikpakJSONResponse(r, 200, `{"captcha_token":"captcha"}`), nil
			case r.Method == http.MethodPost && r.URL.Path == "/drive/v1/files":
				cloudCreates.Add(1)
				return pikpakJSONResponse(r, 200, `{"file":{"id":"video","name":"episode.mkv"}}`), nil
			default:
				return pikpakJSONResponse(r, 200, fmt.Sprintf(`{"id":"video","name":"episode.mkv","size":"%d","kind":"drive#file","web_content_link":%q}`, len(data), media.URL)), nil
			}
		}))
		return client
	}
	a := libraryApp(t)
	a.configMgr.Data.Danmaku.Enabled = false
	a.pikpakClient = newClient()
	var restarted *App
	t.Cleanup(func() {
		if restarted != nil {
			restarted.shutdown(context.Background())
		}
		a.shutdown(context.Background())
		media.Close()
	})
	if result := a.FollowLocal(bangumi.Subject{ID: 42, Name: "Test"}); result != "Success" {
		t.Fatal(result)
	}
	if result := a.DownloadEpisode(42, 1, "magnet:?xt=urn:btih:example"); result != "Started" {
		t.Fatal(result)
	}
	path := filepath.Join(a.dataDir(), "Downloads", "Test_42", "1_episode.mkv")
	deadline := time.Now().Add(5 * time.Second)
	verified := false
	for time.Now().Before(deadline) {
		raw, _ := os.ReadFile(path + ".ua.resume.json")
		var checkpoint struct {
			Completed map[string]string `json:"completed"`
		}
		if json.Unmarshal(raw, &checkpoint) == nil && checkpoint.Completed["0"] != "" {
			verified = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !verified {
		t.Fatal("first block never reached its durable checkpoint")
	}
	if result := a.PauseDownload("42:1"); result != "Success" {
		t.Fatal(result)
	}
	waitDownload(t, a, "paused")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial video became visible", err)
	}
	a.shutdown(context.Background())
	// A new App and API client read only persisted state, as on process restart.
	restarted = &App{configMgr: &config.Manager{ConfigPath: a.configMgr.ConfigPath, Data: a.configMgr.Snapshot()}, webEvents: newWebEventHub(), pikpakClient: newClient()}
	tasks, err := restarted.GetDownloadTasks()
	if err != nil || len(tasks) != 1 || tasks[0].State != "paused" {
		t.Fatalf("restart tasks=%+v error=%v", tasks, err)
	}
	finish.Store(true)
	if result := restarted.RetryDownload("42:1"); result != "Started" {
		t.Fatal(result)
	}
	task := waitDownload(t, restarted, "completed")
	got, err := os.ReadFile(task.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("restored video differs from source", err)
	}
	if firstBlockRequests.Load() != 1 || cloudCreates.Load() != 1 {
		t.Fatalf("redownloaded complete block=%d recreated cloud task=%d", firstBlockRequests.Load(), cloudCreates.Load())
	}
	list, err := restarted.readLibrary()
	if err != nil || list[0].LocalFiles["1"] != path {
		t.Fatal("video index not restored", err)
	}
	for _, suffix := range []string{".ua.part", ".ua.resume.json", ".ua.resume.json.bak"} {
		if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
			t.Fatal("resume artifact not removed", suffix, err)
		}
	}
}
