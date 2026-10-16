package bilibili

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type QRChallenge struct {
	Key       string `json:"key"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

type LoginPoll struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

type LoginStatus struct {
	LoggedIn bool   `json:"logged_in"`
	Username string `json:"username"`
	UserID   int64  `json:"user_id"`
}

func (c *Client) GenerateQRCode(ctx context.Context) (QRChallenge, error) {
	var data struct {
		URL string `json:"url"`
		Key string `json:"qrcode_key"`
	}
	if err := c.api(ctx, "/x/passport-login/web/qrcode/generate", nil, &data); err != nil {
		return QRChallenge{}, err
	}
	address, err := url.Parse(data.URL)
	if err != nil || address.Scheme != "https" || (address.Host != "passport.bilibili.com" && address.Host != "account.bilibili.com") || data.Key == "" {
		return QRChallenge{}, fmt.Errorf("B 站未返回有效的登录二维码")
	}
	return QRChallenge{Key: data.Key, URL: data.URL, ExpiresAt: time.Now().Add(180 * time.Second).Unix()}, nil
}

// PollQRCode keeps credentials in the backend. Neither the cross-domain login
// URL nor the refresh token is returned to the UI or included in an error.
func (c *Client) PollQRCode(ctx context.Context, key string) (LoginPoll, string, error) {
	if key == "" || len(key) > 128 {
		return LoginPoll{}, "", fmt.Errorf("无效的扫码会话")
	}
	var data struct {
		Code int `json:"code"`
	}
	if err := c.api(ctx, "/x/passport-login/web/qrcode/poll", url.Values{"qrcode_key": {key}}, &data); err != nil {
		return LoginPoll{}, "", err
	}
	switch data.Code {
	case 86101:
		return LoginPoll{State: "waiting", Message: "请使用 B 站手机 App 扫码"}, "", nil
	case 86090:
		return LoginPoll{State: "scanned", Message: "已扫码，请在手机上确认登录"}, "", nil
	case 86038:
		return LoginPoll{State: "expired", Message: "二维码已过期，请刷新"}, "", nil
	case 0:
		base, _ := url.Parse(c.baseURL)
		cookies := c.http.Jar.Cookies(base)
		var fields []string
		hasSession, hasCSRF := false, false
		for _, cookie := range cookies {
			if cookie.Name == "SESSDATA" && cookie.Value != "" {
				hasSession = true
			}
			if cookie.Name == "bili_jct" && cookie.Value != "" {
				hasCSRF = true
			}
			fields = append(fields, cookie.Name+"="+cookie.Value)
		}
		if !hasSession || !hasCSRF {
			return LoginPoll{}, "", fmt.Errorf("B 站确认登录后未返回完整 Cookie，请重新扫码")
		}
		sort.Strings(fields)
		return LoginPoll{State: "success", Message: "登录成功，已保存登录信息"}, strings.Join(fields, "; "), nil
	default:
		return LoginPoll{}, "", fmt.Errorf("B 站扫码状态异常（%d），请刷新二维码", data.Code)
	}
}

func (c *Client) LoginStatus(ctx context.Context) (LoginStatus, error) {
	var nav struct {
		LoggedIn bool   `json:"isLogin"`
		Username string `json:"uname"`
		UserID   int64  `json:"mid"`
	}
	if err := c.api(ctx, "/x/web-interface/nav", nil, &nav); err != nil {
		return LoginStatus{}, err
	}
	return LoginStatus{LoggedIn: nav.LoggedIn, Username: nav.Username, UserID: nav.UserID}, nil
}
