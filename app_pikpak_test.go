package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"UltimateAnime/pkg/pikpak"
)

type pikpakRoundTripFunc func(*http.Request) (*http.Response, error)

func (f pikpakRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func pikpakJSONResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func TestSpaceShortageClearsAndRetriesSameAccount(t *testing.T) {
	client := pikpak.NewPikPakClient("first@example.com", "password", "")
	posts, clears, switches := 0, 0, 0
	client.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			clears++
			return pikpakJSONResponse(req, 200, `{"files":[]}`), nil
		}
		if req.Method == http.MethodPatch {
			return pikpakJSONResponse(req, 200, `{}`), nil
		}
		posts++
		if posts == 1 {
			return pikpakJSONResponse(req, 200, `{"error_code":8,"error":"file_space_not_enough"}`), nil
		}
		return pikpakJSONResponse(req, 200, `{"task":{"id":"task-1"}}`), nil
	}))
	a := &App{webEvents: newWebEventHub()}
	task, usedClient, err := a.addOfflineTaskWithRecovery(client, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		switches++
		return nil, nil
	})
	if err != nil || task == nil || task.ID != "task-1" || usedClient != client || posts != 2 || clears != 1 || switches != 0 {
		t.Fatalf("task=%+v client=%p posts=%d clears=%d switches=%d err=%v", task, usedClient, posts, clears, switches, err)
	}
	if a.isAccountBlocked(client.Username) {
		t.Fatal("account with remaining quota should not be blocked")
	}
}

func TestSpaceAndQuotaShortageClearsBeforeSwitching(t *testing.T) {
	first := pikpak.NewPikPakClient("first@example.com", "password", "")
	second := pikpak.NewPikPakClient("second@example.com", "password", "")
	posts, clears, switches := 0, 0, 0
	first.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			clears++
			return pikpakJSONResponse(req, 200, `{"files":[]}`), nil
		}
		if req.Method == http.MethodPatch {
			return pikpakJSONResponse(req, 200, `{}`), nil
		}
		posts++
		if posts == 1 {
			return pikpakJSONResponse(req, 200, `{"error_code":8,"error":"file_space_not_enough"}`), nil
		}
		return pikpakJSONResponse(req, 200, `{"error_code":11,"error":"task_daily_create_limit"}`), nil
	}))
	second.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		posts++
		return pikpakJSONResponse(req, 200, `{"task":{"id":"task-2"}}`), nil
	}))
	a := &App{webEvents: newWebEventHub()}
	task, usedClient, err := a.addOfflineTaskWithRecovery(first, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		if clears != 1 || posts != 2 {
			t.Fatal("account switched before cleanup and retry")
		}
		switches++
		return second, nil
	})
	if err != nil || task == nil || task.ID != "task-2" || usedClient != second || posts != 3 || clears != 1 || switches != 1 {
		t.Fatalf("task=%+v client=%p posts=%d clears=%d switches=%d err=%v", task, usedClient, posts, clears, switches, err)
	}
	if !a.isAccountBlocked(first.Username) {
		t.Fatal("account with exhausted quota should be blocked")
	}
}

func TestPersistentSpaceShortageDoesNotSwitchAccounts(t *testing.T) {
	client := pikpak.NewPikPakClient("first@example.com", "password", "")
	posts, clears, switches := 0, 0, 0
	client.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			clears++
			return pikpakJSONResponse(req, 200, `{"files":[]}`), nil
		}
		if req.Method == http.MethodPatch {
			return pikpakJSONResponse(req, 200, `{}`), nil
		}
		posts++
		return pikpakJSONResponse(req, 200, `{"error_code":8,"error":"file_space_not_enough"}`), nil
	}))
	a := &App{webEvents: newWebEventHub()}
	_, _, err := a.addOfflineTaskWithRecovery(client, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		switches++
		return nil, nil
	})
	if err == nil || posts != 2 || clears != 1 || switches != 0 {
		t.Fatalf("posts=%d clears=%d switches=%d err=%v", posts, clears, switches, err)
	}
}

func TestQuotaErrorWithFullStorageClearsBeforeSwitching(t *testing.T) {
	first := pikpak.NewPikPakClient("first@example.com", "password", "")
	second := pikpak.NewPikPakClient("second@example.com", "password", "")
	aboutCalls, clears, switches := 0, 0, 0
	first.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			if req.URL.Path == "/drive/v1/about" {
				aboutCalls++
				return pikpakJSONResponse(req, 200, `{"quota":{"limit":"100","usage":"100"}}`), nil
			}
			clears++
			return pikpakJSONResponse(req, 200, `{"files":[]}`), nil
		}
		if req.Method == http.MethodPatch {
			return pikpakJSONResponse(req, 200, `{}`), nil
		}
		return pikpakJSONResponse(req, 200, `{"error_code":11,"error":"task_daily_create_limit"}`), nil
	}))
	second.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return pikpakJSONResponse(req, 200, `{"task":{"id":"task-2"}}`), nil
	}))
	a := &App{webEvents: newWebEventHub()}
	task, usedClient, err := a.addOfflineTaskWithRecovery(first, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		if clears != 1 {
			t.Fatal("full cloud was not cleared before account switch")
		}
		switches++
		return second, nil
	})
	if err != nil || task == nil || usedClient != second || aboutCalls != 1 || clears != 1 || switches != 1 {
		t.Fatalf("task=%+v client=%p about=%d clears=%d switches=%d err=%v", task, usedClient, aboutCalls, clears, switches, err)
	}
}

func TestQuotaErrorWithFreeStorageSwitchesWithoutClearing(t *testing.T) {
	first := pikpak.NewPikPakClient("first@example.com", "password", "")
	second := pikpak.NewPikPakClient("second@example.com", "password", "")
	clears := 0
	first.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			if req.URL.Path == "/drive/v1/about" {
				return pikpakJSONResponse(req, 200, `{"quota":{"limit":100,"usage":20}}`), nil
			}
			clears++
			return pikpakJSONResponse(req, 200, `{"files":[]}`), nil
		}
		return pikpakJSONResponse(req, 200, `{"error_code":11,"error":"task_daily_create_limit"}`), nil
	}))
	second.Client.SetTransport(pikpakRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return pikpakJSONResponse(req, 200, `{"task":{"id":"task-2"}}`), nil
	}))
	a := &App{webEvents: newWebEventHub()}
	task, usedClient, err := a.addOfflineTaskWithRecovery(first, "magnet:?xt=urn:btih:example", 2, func() (*pikpak.PikPakClient, error) {
		return second, nil
	})
	if err != nil || task == nil || usedClient != second || clears != 0 {
		t.Fatalf("task=%+v client=%p clears=%d err=%v", task, usedClient, clears, err)
	}
}
