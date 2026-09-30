package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestV01FollowedFileSurvivesReadAndSave(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	legacy := `[{
  "subject_id": 12345,
  "name": "Legacy Anime",
  "name_cn": "旧追番",
  "image": "https://example.com/cover.jpg",
  "air_date": "2025-12-01",
  "added_at": "2025-12-02 12:00:00",
  "watched_eps": [1, 2.5],
  "downloaded_eps": [1],
  "episode_magnets": {"1": "magnet:?xt=urn:btih:example"},
  "local_files": {"1": "Downloads/Legacy Anime/1.mkv"}
}]`
	path := filepath.Join("followed.json")
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	items := a.GetLocalFollows()
	if len(items) != 1 || items[0].SubjectID != 12345 || items[0].WatchedEps[1] != 2.5 || items[0].EpisodeMagnets["1"] == "" || items[0].LocalFiles["1"] == "" {
		t.Fatalf("old followed item was not loaded: %+v", items)
	}
	if result := a.saveFollowedList(items); result != "Success" {
		t.Fatal(result)
	}
	reloaded := a.GetLocalFollows()
	if len(reloaded) != 1 || reloaded[0].WatchedEps[1] != 2.5 || reloaded[0].EpisodeMagnets["1"] != items[0].EpisodeMagnets["1"] || reloaded[0].LocalFiles["1"] != items[0].LocalFiles["1"] {
		t.Fatalf("old followed data was not preserved after save: %+v", reloaded)
	}
}
