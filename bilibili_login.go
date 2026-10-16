package main

import (
	"context"
	"fmt"
	"time"

	"UltimateAnime/pkg/bilibili"
)

type bilibiliQRClient interface {
	PollQRCode(context.Context, string) (bilibili.LoginPoll, string, error)
}

type bilibiliLoginSession struct {
	client    bilibiliQRClient
	key       string
	expiresAt int64
}

func (a *App) bilibiliContext() (context.Context, context.CancelFunc) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 30*time.Second)
}

func (a *App) newBilibiliClient() (*bilibili.Client, error) {
	if a.configMgr == nil {
		return nil, fmt.Errorf("配置尚未初始化")
	}
	cfg := a.configMgr.Snapshot()
	return bilibili.NewClient(cfg.GlobalSettings.Proxy, cfg.Danmaku.BilibiliCookie)
}

func (a *App) StartBilibiliLogin() (bilibili.QRChallenge, error) {
	if a.configMgr == nil {
		return bilibili.QRChallenge{}, fmt.Errorf("配置尚未初始化")
	}
	a.bilibiliLoginMu.Lock()
	defer a.bilibiliLoginMu.Unlock()
	a.bilibiliLogin = nil
	client, err := bilibili.NewClient(a.configMgr.GetProxy(), "")
	if err != nil {
		return bilibili.QRChallenge{}, err
	}
	ctx, cancel := a.bilibiliContext()
	defer cancel()
	challenge, err := client.GenerateQRCode(ctx)
	if err != nil {
		return bilibili.QRChallenge{}, err
	}
	a.bilibiliLogin = &bilibiliLoginSession{client: client, key: challenge.Key, expiresAt: challenge.ExpiresAt}
	return challenge, nil
}

func (a *App) PollBilibiliLogin(key string) (bilibili.LoginPoll, error) {
	a.bilibiliLoginMu.Lock()
	session := a.bilibiliLogin
	if session == nil || session.key != key {
		a.bilibiliLoginMu.Unlock()
		return bilibili.LoginPoll{}, fmt.Errorf("扫码会话已取消或被替换")
	}
	if time.Now().Unix() >= session.expiresAt {
		a.bilibiliLogin = nil
		a.bilibiliLoginMu.Unlock()
		return bilibili.LoginPoll{State: "expired", Message: "二维码已过期，请刷新"}, nil
	}
	a.bilibiliLoginMu.Unlock()
	ctx, cancel := a.bilibiliContext()
	defer cancel()
	status, cookie, err := session.client.PollQRCode(ctx, key)
	a.bilibiliLoginMu.Lock()
	defer a.bilibiliLoginMu.Unlock()
	if a.bilibiliLogin != session {
		return bilibili.LoginPoll{}, fmt.Errorf("扫码会话已取消或被替换")
	}
	if err != nil {
		return bilibili.LoginPoll{}, err
	}
	if status.State == "success" {
		if err := a.configMgr.UpdateBilibiliCookie(cookie); err != nil {
			return bilibili.LoginPoll{}, fmt.Errorf("保存 B 站登录信息失败")
		}
		a.bilibiliLogin = nil
		a.Log("SUCCESS", "B 站扫码登录成功，已保存登录信息")
	} else if status.State == "expired" {
		a.bilibiliLogin = nil
	}
	return status, nil
}

func (a *App) CancelBilibiliLogin(key string) {
	a.bilibiliLoginMu.Lock()
	defer a.bilibiliLoginMu.Unlock()
	if a.bilibiliLogin != nil && a.bilibiliLogin.key == key {
		a.bilibiliLogin = nil
	}
}

func (a *App) GetBilibiliLoginStatus() (bilibili.LoginStatus, error) {
	client, err := a.newBilibiliClient()
	if err != nil {
		return bilibili.LoginStatus{}, err
	}
	if a.configMgr.Snapshot().Danmaku.BilibiliCookie == "" {
		return bilibili.LoginStatus{}, nil
	}
	ctx, cancel := a.bilibiliContext()
	defer cancel()
	return client.LoginStatus(ctx)
}

func (a *App) ClearBilibiliLogin() string {
	a.bilibiliLoginMu.Lock()
	defer a.bilibiliLoginMu.Unlock()
	a.bilibiliLogin = nil
	if a.configMgr == nil {
		return "Error: 配置尚未初始化"
	}
	if err := a.configMgr.UpdateBilibiliCookie(""); err != nil {
		return "Error: 清除 B 站登录信息失败"
	}
	return "Success"
}
