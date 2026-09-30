package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"UltimateAnime/pkg/config"
	"UltimateAnime/pkg/pikpak"
)

func TestSavingPikPakCredentialsInvalidatesConnection(t *testing.T) {
	mgr := &config.Manager{ConfigPath: filepath.Join(t.TempDir(), "config.json"), Data: config.NewDefaultConfig()}
	a := &App{
		configMgr:    mgr,
		pikpakClient: pikpak.NewPikPakClient("old@example.com", "old", ""),
		webEvents:    newWebEventHub(),
	}
	updated := mgr.Snapshot()
	updated.GlobalSettings.PikPakUsers = []string{"new@example.com"}
	updated.GlobalSettings.PikPakPassword = "new"
	encoded, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	if result := a.SaveAppConfig(string(encoded)); result != "Success" {
		t.Fatal(result)
	}
	if status := a.GetPikPakStatus(); status != "未登录" {
		t.Fatalf("status after credential change: %s", status)
	}
	if got := mgr.Snapshot().GlobalSettings.PikPakUsers; len(got) != 1 || got[0] != "new@example.com" {
		t.Fatalf("saved account was not applied: %v", got)
	}
}

func TestDownloadEpisodeRequiresCredentialsBeforeStarting(t *testing.T) {
	mgr := &config.Manager{ConfigPath: filepath.Join(t.TempDir(), "config.json"), Data: config.NewDefaultConfig()}
	a := &App{configMgr: mgr, webEvents: newWebEventHub()}
	if result := a.DownloadEpisode(42, 1, "magnet:?xt=urn:btih:example"); !strings.Contains(result, "配置 PikPak 账号") {
		t.Fatalf("expected configuration error, got %s", result)
	}
}

func TestAutoSelectMagnetDefaultsToManualAndAppliesOnSave(t *testing.T) {
	mgr := &config.Manager{ConfigPath: filepath.Join(t.TempDir(), "config.json"), Data: config.NewDefaultConfig()}
	a := &App{configMgr: mgr, webEvents: newWebEventHub()}
	if a.GetAutoSelectMagnet() {
		t.Fatal("existing config should keep manual selection")
	}

	updated := mgr.Snapshot()
	updated.TorrentSearcher.AutoSelectMagnet = true
	encoded, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	if result := a.SaveAppConfig(string(encoded)); result != "Success" {
		t.Fatal(result)
	}
	if !a.GetAutoSelectMagnet() {
		t.Fatal("saved setting was not applied")
	}

	loaded := &config.Manager{ConfigPath: mgr.ConfigPath, Data: config.NewDefaultConfig()}
	if err := loaded.Load(); err != nil || !loaded.Snapshot().TorrentSearcher.AutoSelectMagnet {
		t.Fatalf("saved setting was not persisted: %v", err)
	}
}
