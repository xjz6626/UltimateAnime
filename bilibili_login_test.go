package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"UltimateAnime/pkg/bilibili"
)

type fakeQRClient struct {
	poll func() (bilibili.LoginPoll, string, error)
}

func (f fakeQRClient) PollQRCode(context.Context, string) (bilibili.LoginPoll, string, error) {
	return f.poll()
}

func TestBilibiliQRSuccessPersistsOnlySessionAndKeepsOtherSettings(t *testing.T) {
	app, _ := danmakuTestApp(t)
	cfg := app.configMgr.Snapshot()
	cfg.Player.MPVArgs = "--fullscreen"
	cfg.Danmaku.AutoSelect = true
	if err := app.configMgr.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	app.bilibiliLogin = &bilibiliLoginSession{key: "session", expiresAt: time.Now().Add(time.Minute).Unix(), client: fakeQRClient{poll: func() (bilibili.LoginPoll, string, error) {
		return bilibili.LoginPoll{State: "success"}, "SESSDATA=secret; bili_jct=csrf", nil
	}}}
	status, err := app.PollBilibiliLogin("session")
	if err != nil || status.State != "success" {
		t.Fatalf("status=%+v error=%v", status, err)
	}
	updated := app.configMgr.Snapshot()
	if updated.Danmaku.BilibiliCookie != "SESSDATA=secret; bili_jct=csrf" || !updated.Danmaku.AutoSelect || updated.Player.MPVArgs != "--fullscreen" {
		t.Fatal("login overwrote other settings")
	}
	for _, log := range app.GetLogs() {
		if strings.Contains(log["message"], "secret") {
			t.Fatal("secret appeared in logs")
		}
	}
	encoded, _ := json.Marshal(status)
	if strings.Contains(string(encoded), "secret") {
		t.Fatal("secret appeared in status")
	}
	persisted, err := os.ReadFile(app.configMgr.ConfigPath)
	if err != nil || !strings.Contains(string(persisted), "SESSDATA=secret") {
		t.Fatal("session was not persisted", err)
	}
	if app.bilibiliLogin != nil {
		t.Fatal("completed session was retained")
	}
}

func TestCancelBilibiliLoginDiscardsLateConfirmation(t *testing.T) {
	app, _ := danmakuTestApp(t)
	started, finish := make(chan struct{}), make(chan struct{})
	app.bilibiliLogin = &bilibiliLoginSession{key: "session", expiresAt: time.Now().Add(time.Minute).Unix(), client: fakeQRClient{poll: func() (bilibili.LoginPoll, string, error) {
		close(started)
		<-finish
		return bilibili.LoginPoll{State: "success"}, "SESSDATA=late-secret", nil
	}}}
	done := make(chan error, 1)
	go func() { _, err := app.PollBilibiliLogin("session"); done <- err }()
	<-started
	app.CancelBilibiliLogin("session")
	close(finish)
	if err := <-done; err == nil {
		t.Fatal("late confirmation accepted after cancellation")
	}
	if app.configMgr.Snapshot().Danmaku.BilibiliCookie != "" {
		t.Fatal("cancelled QR saved a session")
	}
}

func TestBilibiliQRExpiryAndClear(t *testing.T) {
	app, _ := danmakuTestApp(t)
	app.bilibiliLogin = &bilibiliLoginSession{key: "expired", expiresAt: time.Now().Add(-time.Minute).Unix(), client: fakeQRClient{poll: func() (bilibili.LoginPoll, string, error) {
		return bilibili.LoginPoll{}, "", errors.New("must not poll expired session")
	}}}
	status, err := app.PollBilibiliLogin("expired")
	if err != nil || status.State != "expired" {
		t.Fatal(status, err)
	}
	if err := app.configMgr.UpdateBilibiliCookie("SESSDATA=secret"); err != nil {
		t.Fatal(err)
	}
	if result := app.ClearBilibiliLogin(); result != "Success" || app.configMgr.Snapshot().Danmaku.BilibiliCookie != "" {
		t.Fatal(result)
	}
}
