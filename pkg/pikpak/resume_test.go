package pikpak

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resumeTestClient() *PikPakClient {
	client := NewPikPakClient("owner", "password", "")
	client.Client.SetTransport(mockTransport(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "captcha") {
			return jsonResponse(req, 200, `{"captcha_token":"captcha"}`), nil
		}
		return jsonResponse(req, 200, `{"web_content_link":"https://cdn.example.test/file"}`), nil
	}))
	return client
}

func TestResumeAcrossClientsVerifiesBlocksAndPreservesDestination(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint("corrupt=", corrupt), func(t *testing.T) {
			data := bytes.Repeat([]byte("abcde"), int(downloadBlockSize/5)+100)
			path := filepath.Join(t.TempDir(), "episode.mkv")
			if err := os.WriteFile(path, []byte("previous complete video"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			requests := []int64{}
			transport := mockTransport(func(req *http.Request) (*http.Response, error) {
				var start, end int64
				if _, err := fmt.Sscanf(req.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
					return nil, err
				}
				requests = append(requests, start)
				if start > 0 {
					cancel()
					return nil, ctx.Err()
				}
				return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", start, end, len(data))}}, Body: io.NopCloser(bytes.NewReader(data[start : end+1])), Request: req}, nil
			})
			err := resumeTestClient().downloadFileConcurrent("file", path, int64(len(data)), 1, nil, downloadOptions{context: ctx, resume: true, rangeTransport: transport})
			if err == nil {
				t.Fatal("cancelled download succeeded")
			}
			old, _ := os.ReadFile(path)
			if string(old) != "previous complete video" {
				t.Fatal("incomplete download replaced destination")
			}
			if corrupt {
				file, err := os.OpenFile(path+".ua.part", os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.WriteAt([]byte("corruption"), 100)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			requests = nil
			transport = mockTransport(func(req *http.Request) (*http.Response, error) {
				var start, end int64
				if _, err := fmt.Sscanf(req.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
					return nil, err
				}
				requests = append(requests, start)
				return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", start, end, len(data))}}, Body: io.NopCloser(bytes.NewReader(data[start : end+1])), Request: req}, nil
			})
			var firstProgress int64 = -1
			err = resumeTestClient().downloadFileConcurrent("file", path, int64(len(data)), 1, func(current, _ int64) {
				if firstProgress == -1 {
					firstProgress = current
				}
			}, downloadOptions{resume: true, rangeTransport: transport})
			if err != nil {
				t.Fatal(err)
			}
			wantRequests, wantProgress := 1, downloadBlockSize
			if corrupt {
				wantRequests, wantProgress = 2, 0
			}
			if len(requests) != wantRequests || firstProgress != wantProgress {
				t.Fatalf("requests=%v initial progress=%d", requests, firstProgress)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatal("resumed content mismatch", err)
			}
			for _, suffix := range []string{".ua.part", ".ua.resume.json", ".ua.resume.json.bak"} {
				if _, err := os.Stat(path + suffix); !os.IsNotExist(err) {
					t.Fatal("left resume artifact", suffix, err)
				}
			}
		})
	}
}

func TestResumeRejectsWrongIdentityAndCancelPreservesVideo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mkv")
	r, err := openResumeFile(context.Background(), path, "owner:file1", 5)
	if err != nil {
		t.Fatal(err)
	}
	r.file.Close()
	if other, err := openResumeFile(context.Background(), path, "owner:file2", 5); err == nil {
		other.file.Close()
		t.Fatal("accepted different resource")
	}
	if err := os.WriteFile(path, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := DiscardPartial(path); err != nil {
		t.Fatal(err)
	}
	if err := DiscardPartial(path); err != nil {
		t.Fatal("cancel is not idempotent", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "video" {
		t.Fatal("cancel removed completed video")
	}
}

func TestDownloadRejectsWrongContentRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mkv")
	transport := mockTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Range": {"bytes 5-9/10"}}, Body: io.NopCloser(strings.NewReader("hello")), Request: req}, nil
	})
	err := resumeTestClient().downloadFileConcurrent("file", path, 5, 1, nil, downloadOptions{resume: true, rangeTransport: transport})
	if err == nil || !strings.Contains(err.Error(), "Content-Range") {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("incorrect range promoted to video")
	}
}
