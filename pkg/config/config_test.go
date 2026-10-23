package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadV01ConfigWithoutNewFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := `{
  "global_settings": {
    "bangumi_api_token": "old-token",
    "pikpak_users": ["old@example.com"],
    "pikpak_password": "old-password",
    "auto_login": true,
    "proxy": "http://127.0.0.1:7897"
  },
  "local_storage": {"anime_dir": "Downloads"},
  "torrent_searcher": {"watchlist_file": "followed.json"},
  "player": {"mpv_path": "C:\\mpv\\mpv.exe", "mpv_args": "--fullscreen"}
}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	mgr := &Manager{ConfigPath: path, Data: NewDefaultConfig()}
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}
	got := mgr.Snapshot()
	if got.GlobalSettings.PikPakUsers[0] != "old@example.com" || got.GlobalSettings.PikPakPassword != "old-password" || got.GlobalSettings.Proxy != "http://127.0.0.1:7897" {
		t.Fatalf("legacy credentials or proxy changed: %+v", got.GlobalSettings)
	}
	if got.Player.MPVPath != `C:\mpv\mpv.exe` || got.TorrentSearcher.AutoSelectMagnet {
		t.Fatalf("legacy player or manual magnet mode changed: %+v, %+v", got.Player, got.TorrentSearcher)
	}
	if !got.Danmaku.Enabled || got.Danmaku.AutoSelect || got.Danmaku.BilibiliCookie != "" {
		t.Fatalf("legacy config should enable anonymous danmaku by default: %+v", got.Danmaku)
	}
	if err := mgr.Save(); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"pikpak_password": "old-password"`) || !strings.Contains(string(saved), `"auto_select_magnet": false`) {
		t.Fatal("upgraded config did not retain legacy values and current defaults")
	}
}

func TestCorruptConfigIsReportedAndPreserved(t *testing.T) {
	for _, data := range []string{"{broken", "null", "[]"} {
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, []byte(data), 0600)
		mgr := &Manager{ConfigPath: path, Data: NewDefaultConfig()}
		if err := mgr.Load(); err == nil || mgr.LoadError() == nil {
			t.Fatal("load error hidden")
		}
		if err := mgr.Replace(NewDefaultConfig()); err == nil {
			t.Fatal("corrupt file overwritten")
		}
		if err := mgr.UpdateBilibiliCookie("cookie"); err == nil {
			t.Fatal("cookie save overwrote corrupt file")
		}
		after, _ := os.ReadFile(path)
		if string(after) != data {
			t.Fatal("source changed")
		}
	}
}

func TestSnapshotAndAccountStateSurviveRestart(t *testing.T) {
	mgr := &Manager{ConfigPath: filepath.Join(t.TempDir(), "config.json"), Data: NewDefaultConfig()}
	disabled := true
	if err := mgr.SetAccountState("one", &disabled, ""); err != nil {
		t.Fatal(err)
	}
	snapshot := mgr.Snapshot()
	snapshot.GlobalSettings.DisabledAccounts["one"] = false
	if !mgr.Snapshot().GlobalSettings.DisabledAccounts["one"] {
		t.Fatal("snapshot aliases live state")
	}
	loaded := &Manager{ConfigPath: mgr.ConfigPath}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if !loaded.Snapshot().GlobalSettings.DisabledAccounts["one"] {
		t.Fatal("disable was not persistent")
	}
}
