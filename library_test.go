package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"UltimateAnime/pkg/bangumi"
	"UltimateAnime/pkg/config"
)

func libraryApp(t *testing.T) *App {
	t.Helper()
	return &App{configMgr: &config.Manager{ConfigPath: filepath.Join(t.TempDir(), "config.json"), Data: config.NewDefaultConfig()}, webEvents: newWebEventHub()}
}

func TestLibraryConcurrentTransactionsAndArchive(t *testing.T) {
	a := libraryApp(t)
	var group sync.WaitGroup
	for id := 1; id <= 40; id++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			if result := a.FollowLocal(bangumi.Subject{ID: id, Name: fmt.Sprint(id)}); result != "Success" {
				t.Error(result)
			}
		}(id)
	}
	group.Wait()
	for i := 1; i <= 30; i++ {
		group.Add(1)
		go func(ep float64) {
			defer group.Done()
			if result := a.ToggleEpisodeWatched(1, ep); result != "Success" {
				t.Error(result)
			}
			if result := a.SaveEpisodeMagnet(1, ep, fmt.Sprint(ep)); result != "Success" {
				t.Error(result)
			}
			if err := a.markEpisodeDownloaded(1, ep, fmt.Sprint(ep)); err != nil {
				t.Error(err)
			}
		}(float64(i))
	}
	group.Add(1)
	go func() {
		defer group.Done()
		if result := a.UnfollowLocal(1); result != "Success" {
			t.Error(result)
		}
	}()
	group.Wait()
	follows, err := a.GetLocalFollowsChecked()
	if err != nil || len(follows) != 39 {
		t.Fatalf("follows=%d err=%v", len(follows), err)
	}
	if result := a.FollowLocal(bangumi.Subject{ID: 1}); result != "Success" {
		t.Fatal(result)
	}
	list, err := a.readLibrary()
	if err != nil || len(list) != 40 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	for _, item := range list {
		if item.SubjectID == 1 {
			if len(item.WatchedEps) != 30 || len(item.DownloadedEps) != 30 || len(item.LocalFiles) != 30 || len(item.EpisodeMagnets) != 30 {
				t.Fatalf("lost data: %+v", item)
			}
		}
	}
}

func TestCorruptLibraryNeverOverwritten(t *testing.T) {
	for _, input := range []string{"{broken", " null ", "{}"} {
		t.Run(input, func(t *testing.T) {
			a := libraryApp(t)
			path := filepath.Join(a.dataDir(), "followed.json")
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := a.GetLocalFollowsChecked(); err == nil {
				t.Fatal("corruption hidden")
			}
			if result := a.FollowLocal(bangumi.Subject{ID: 1}); !strings.HasPrefix(result, "Error:") {
				t.Fatal(result)
			}
			data, _ := os.ReadFile(path)
			if string(data) != input {
				t.Fatal("corrupt source overwritten")
			}
		})
	}
}

func TestDeleteFailurePreservesAllEpisodeIndexes(t *testing.T) {
	a := libraryApp(t)
	a.FollowLocal(bangumi.Subject{ID: 1, Name: "Test"})
	directory := filepath.Join(a.dataDir(), "nonempty")
	os.Mkdir(directory, 0700)
	os.WriteFile(filepath.Join(directory, "file"), []byte("video"), 0600)
	a.markEpisodeDownloaded(1, 1, directory)
	a.SaveEpisodeMagnet(1, 1, "magnet:test")
	if result := a.DeleteEpisodeData(1, 1); !strings.Contains(result, "记录已保留") {
		t.Fatal(result)
	}
	list, _ := a.readLibrary()
	if list[0].LocalFiles["1"] != directory || len(list[0].DownloadedEps) != 1 || list[0].EpisodeMagnets["1"] == "" {
		t.Fatalf("lost indexes: %+v", list)
	}
	os.Remove(filepath.Join(directory, "file"))
	if result := a.DeleteEpisodeData(1, 1); result != "Success" {
		t.Fatal(result)
	}
}

func TestWriteFailureKeepsLibraryAndReportsError(t *testing.T) {
	a := libraryApp(t)
	a.FollowLocal(bangumi.Subject{ID: 1, Name: "Test"})
	path := filepath.Join(a.dataDir(), "followed.json")
	before, _ := os.ReadFile(path)
	// A directory at the backup target injects a deterministic rename failure.
	os.Mkdir(path+".bak", 0700)
	if result := a.ToggleEpisodeWatched(1, 1); !strings.HasPrefix(result, "Error:") {
		t.Fatal(result)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("data modified after failure")
	}
	if err := a.markEpisodeDownloaded(1, 1, "file"); err == nil {
		t.Fatal("download index failure hidden")
	}
}

func TestMissingFileRecoveryPreservesOtherErrors(t *testing.T) {
	a := libraryApp(t)
	a.FollowLocal(bangumi.Subject{ID: 1, Name: "Test"})
	file := filepath.Join(a.dataDir(), "missing.mkv")
	a.markEpisodeDownloaded(1, 1, file)
	if got := a.PlayLocalEpisode(1, 1); got != "Error: FileMissing" {
		t.Fatal(got)
	}
	list, _ := a.readLibrary()
	if len(list[0].DownloadedEps) != 0 {
		t.Fatal("missing index retained")
	}
	missingDrive := filepath.Join(a.dataDir(), "unmounted", "video.mkv")
	a.markEpisodeDownloaded(1, 1, missingDrive)
	if got := a.PlayLocalEpisode(1, 1); !strings.Contains(got, "记录已保留") {
		t.Fatal(got)
	}
	list, _ = a.readLibrary()
	if len(list[0].DownloadedEps) != 1 {
		t.Fatal("unavailable disk index removed")
	}
}
