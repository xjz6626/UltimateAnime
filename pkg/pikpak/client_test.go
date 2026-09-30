package pikpak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockTransport func(*http.Request) (*http.Response, error)

func (f mockTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestRequestRejectsAPIErrorOnHTTP200(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(req, 200, `{"error_code":8,"error":"file_space_not_enough"}`), nil
	}))
	_, err := client.request("https://api-drive.mypikpak.com/drive/v1/files", http.MethodGet, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "api error 8") {
		t.Fatalf("expected space error, got %v", err)
	}
}

func TestRequestLimitsAuthenticationRetry(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	driveCalls := 0
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/shield/captcha/init":
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		case "/v1/auth/signin":
			return jsonResponse(req, 200, `{"access_token":"token","refresh_token":"refresh"}`), nil
		default:
			driveCalls++
			return jsonResponse(req, 401, `{"error_code":16,"error":"expired"}`), nil
		}
	}))
	_, err := client.request("https://api-drive.mypikpak.com/drive/v1/files", http.MethodGet, nil, nil)
	if err == nil || driveCalls != 2 {
		t.Fatalf("expected one authentication retry, calls=%d error=%v", driveCalls, err)
	}
}

func TestLoginRejectsMissingAccessToken(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "captcha") {
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		}
		return jsonResponse(req, 200, `{}`), nil
	}))
	if err := client.Login(); err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestOfflineDownloadRejectsEmptyTask(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(req, 200, `{}`), nil
	}))
	task, err := client.OfflineDownload("magnet:?xt=urn:btih:example", "", "")
	if task != nil || err == nil {
		t.Fatalf("expected task creation error, task=%+v error=%v", task, err)
	}
}

func TestDeleteTaskIncludesFileChoice(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	var calls int
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodDelete || req.URL.Path != "/drive/v1/tasks" || req.URL.Query().Get("task_ids") != "task-1" {
			t.Errorf("unexpected task deletion request: %s %s", req.Method, req.URL.Path)
		}
		want := "false"
		if calls == 2 {
			want = "true"
		}
		if got := req.URL.Query().Get("delete_files"); got != want {
			t.Errorf("delete_files = %q, want %q", got, want)
		}
		return jsonResponse(req, 200, `{}`), nil
	}))
	if err := client.DeleteTask("task-1", false); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteTask("task-1", true); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("got %d calls, want 2", calls)
	}
}

func TestDeleteTaskRetriesCaptchaWithPathAction(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	deleteCalls, captchaCalls := 0, 0
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/drive/v1/tasks":
			deleteCalls++
			if req.URL.Query().Get("delete_files") != "false" {
				t.Error("task deletion changed delete_files on retry")
			}
			if deleteCalls == 1 {
				return jsonResponse(req, 200, `{"error_code":9,"error":"captcha_required"}`), nil
			}
			if req.Header.Get("X-Captcha-Token") != "captcha" {
				t.Error("captcha token missing from retry")
			}
			return jsonResponse(req, 200, `{}`), nil
		case "/v1/shield/captcha/init":
			captchaCalls++
			var body struct {
				Action string `json:"action"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Action != "DELETE:/drive/v1/tasks" {
				t.Errorf("captcha action = %q", body.Action)
			}
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonResponse(req, 404, `{}`), nil
		}
	}))
	if err := client.DeleteTask("task-1", false); err != nil {
		t.Fatal(err)
	}
	if deleteCalls != 2 || captchaCalls != 1 {
		t.Fatalf("delete calls=%d, captcha calls=%d", deleteCalls, captchaCalls)
	}
}

func TestClearStorageReportsUnremovedFile(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	var deleteCalls, emptyTrashCalls int
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/files":
			if strings.Contains(req.URL.Query().Get("filters"), "phase") {
				t.Error("storage cleanup must include incomplete files")
			}
			return jsonResponse(req, 200, `{"files":[{"id":"file-1","kind":"drive#file"}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/drive/v1/files:batchDelete":
			deleteCalls++
			return jsonResponse(req, 200, `{"error_code":8,"error":"delete_failed"}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/drive/v1/files/trash:empty":
			emptyTrashCalls++
			return jsonResponse(req, 200, `{}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonResponse(req, 404, `{}`), nil
		}
	}))
	if err := client.ClearStorage(); err == nil {
		t.Fatal("expected failure when file remains")
	}
	if deleteCalls != 3 || emptyTrashCalls != 0 {
		t.Fatalf("delete calls=%d, trash empty calls=%d", deleteCalls, emptyTrashCalls)
	}
}

func TestClearStorageEmptiesTrashAfterProtectedFolderContents(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	childDeleted := false
	var emptyTrashCalls int
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/files":
			if req.URL.Query().Get("parent_id") == "system-folder" {
				if childDeleted {
					return jsonResponse(req, 200, `{"files":[]}`), nil
				}
				return jsonResponse(req, 200, `{"files":[{"id":"child-file","kind":"drive#file"}]}`), nil
			}
			return jsonResponse(req, 200, `{"files":[{"id":"system-folder","kind":"drive#folder"}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/drive/v1/files:batchDelete":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "child-file") {
				childDeleted = true
				return jsonResponse(req, 200, `{}`), nil
			}
			return jsonResponse(req, 200, `{"error_code":8,"error":"protected_folder"}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/drive/v1/files/trash:empty":
			emptyTrashCalls++
			return jsonResponse(req, 200, `{}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonResponse(req, 404, `{}`), nil
		}
	}))
	if err := client.ClearStorage(); err != nil {
		t.Fatal(err)
	}
	if !childDeleted || emptyTrashCalls != 1 {
		t.Fatalf("child deleted=%t, trash empty calls=%d", childDeleted, emptyTrashCalls)
	}
}

func TestDownloadFileConcurrentStopsAfterFailedChunks(t *testing.T) {
	requests := 0
	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "captcha") {
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		}
		return jsonResponse(req, 200, `{"web_content_link":"https://cdn.example.test/file"}`), nil
	}))
	path := filepath.Join(t.TempDir(), "episode.mkv")
	rangeTransport := mockTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(req, http.StatusServiceUnavailable, `{"error":"unavailable"}`), nil
	})
	err := client.downloadFileConcurrent("file-id", path, 5, 1, nil, rangeTransport)
	if err == nil || requests != 3 {
		t.Fatalf("expected three attempts and an error, requests=%d error=%v", requests, err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("failed download left a destination file: %v", statErr)
	}
}

func TestDownloadFileConcurrentWritesCompleteFile(t *testing.T) {
	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "captcha") {
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		}
		return jsonResponse(req, 200, `{"web_content_link":"https://cdn.example.test/file"}`), nil
	}))
	path := filepath.Join(t.TempDir(), "episode.mkv")
	rangeTransport := mockTransport(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Range"); got != "bytes=0-4" {
			t.Errorf("range = %q", got)
		}
		return &http.Response{StatusCode: http.StatusPartialContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("hello")), Request: req}, nil
	})
	if err := client.downloadFileConcurrent("file-id", path, 5, 1, nil, rangeTransport); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "hello" {
		t.Fatalf("file contents=%q, err=%v", contents, err)
	}
}

func TestServeStreamForwardsRangeWithoutBrowserCredentials(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("Range"); got != "bytes=0-0" {
			t.Errorf("range = %q", got)
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Error("browser credentials were forwarded to file host")
		}
		w.Header().Set("Content-Range", "bytes 0-0/5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("h"))
	}))
	defer cdn.Close()

	client := NewPikPakClient("test@example.com", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/shield/captcha/init" {
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		}
		return jsonResponse(req, 200, fmt.Sprintf(`{"web_content_link":%q}`, cdn.URL)), nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/stream?id=file-1", nil)
	request.Header.Set("Range", "bytes=0-0")
	request.Header.Set("Authorization", "Bearer browser-secret")
	request.Header.Set("Cookie", "session=browser-secret")
	response := httptest.NewRecorder()
	client.ServeStream(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "h" || response.Header().Get("Content-Range") != "bytes 0-0/5" {
		t.Fatalf("status=%d body=%q content-range=%q", response.Code, response.Body.String(), response.Header().Get("Content-Range"))
	}
}
