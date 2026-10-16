package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQRLoginUsesBilibiliDomainAndSavesHeaderCookies(t *testing.T) {
	client, _ := NewClient("", "")
	codes := []int{86101, 86090, 0}
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "passport.bilibili.com" {
			t.Fatal("login request went to wrong host", r.URL.Host)
		}
		if strings.HasSuffix(r.URL.Path, "/generate") {
			return response(`{"code":0,"data":{"qrcode_key":"test-key","url":"https://account.bilibili.com/h5/account-h5/auth/scan-web?qrcode_key=test-key"}}`), nil
		}
		if r.URL.Query().Get("qrcode_key") != "test-key" {
			t.Fatal("missing login session key")
		}
		code := codes[0]
		codes = codes[1:]
		resp := response(`{"code":0,"data":{"code":` + stringInt(code) + `,"url":"private-login-url","refresh_token":"private-refresh-token"}}`)
		if code == 0 {
			resp.Header.Add("Set-Cookie", "SESSDATA=private-session; Path=/; Domain=bilibili.com; Secure; HttpOnly")
			resp.Header.Add("Set-Cookie", "bili_jct=private-csrf; Path=/; Domain=bilibili.com; Secure")
		}
		return resp, nil
	})
	challenge, err := client.GenerateQRCode(context.Background())
	if err != nil || challenge.Key != "test-key" || challenge.ExpiresAt < time.Now().Add(175*time.Second).Unix() {
		t.Fatalf("challenge=%+v err=%v", challenge, err)
	}
	for _, expected := range []string{"waiting", "scanned", "success"} {
		status, cookie, err := client.PollQRCode(context.Background(), challenge.Key)
		if err != nil || status.State != expected {
			t.Fatalf("status=%+v err=%v", status, err)
		}
		if expected == "success" && (!strings.Contains(cookie, "SESSDATA=private-session") || !strings.Contains(cookie, "bili_jct=private-csrf")) {
			t.Fatal("login cookies were not captured")
		}
		encoded, _ := json.Marshal(status)
		if strings.Contains(string(encoded), "private") {
			t.Fatal("poll status leaked credentials")
		}
	}
}

func stringInt(value int) string { data, _ := json.Marshal(value); return string(data) }

func TestQRLoginRejectsForeignURLAndIncompleteSuccess(t *testing.T) {
	client, _ := NewClient("", "")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/generate") {
			return response(`{"code":0,"data":{"qrcode_key":"x","url":"https://attacker.example/scan"}}`), nil
		}
		return response(`{"code":0,"data":{"code":0,"url":"private-session-in-url"}}`), nil
	})
	if _, err := client.GenerateQRCode(context.Background()); err == nil {
		t.Fatal("foreign QR URL accepted")
	}
	if _, _, err := client.PollQRCode(context.Background(), "x"); err == nil || strings.Contains(err.Error(), "private-session") {
		t.Fatal("incomplete login must fail without exposing server data", err)
	}
}

func TestQRLoginExpiration(t *testing.T) {
	client, _ := NewClient("", "")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(`{"code":0,"data":{"code":86038}}`), nil
	})
	status, cookie, err := client.PollQRCode(context.Background(), "x")
	if err != nil || status.State != "expired" || cookie != "" {
		t.Fatalf("status=%+v cookie=%q err=%v", status, cookie, err)
	}
}
