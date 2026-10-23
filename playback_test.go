package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"UltimateAnime/pkg/bangumi"
)

func playbackFixture(t *testing.T) (*App, string, PlaybackProgress) {
	t.Helper()
	a := libraryApp(t)
	a.FollowLocal(bangumi.Subject{ID: 42, Name: "Test"})
	path := filepath.Join(a.dataDir(), "episode with spaces.wav")
	if err := os.WriteFile(path, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.markEpisodeDownloaded(42, 1, path); err != nil {
		t.Fatal(err)
	}
	version, err := mediaVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	p := PlaybackProgress{SessionID: "session", MediaVersion: version, Position: 30, Duration: 100, UpdatedAt: time.Now(), Ranges: []ViewedRange{{0, 30}}}
	if err := a.savePlayback(42, 1, path, p, false); err != nil {
		t.Fatal(err)
	}
	return a, path, p
}

func property(name string, value any) mpvEvent {
	data, _ := json.Marshal(value)
	return mpvEvent{Event: "property-change", Name: name, Data: data}
}

func TestPlaybackCoverageDoesNotCountSeekingPausesOrRepeatedSpans(t *testing.T) {
	tracker := playbackTracker{path: "video", speed: 1}
	now := time.Unix(1000, 0)
	send := func(name string, value any, after time.Duration) {
		now = now.Add(after)
		tracker.consume(property(name, value), now)
	}
	send("path", "video", 0)
	send("duration", 100, 0)
	send("time-pos", 0, 0)
	send("time-pos", 10, 10*time.Second)
	// The seek jump is not viewing; only the following second is.
	send("seeking", true, 0)
	send("time-pos", 90, time.Second)
	send("seeking", false, 0)
	send("time-pos", 91, time.Second)
	send("pause", true, 0)
	send("time-pos", 95, 10*time.Second)
	send("pause", false, 0)
	send("time-pos", 95, 0)
	send("time-pos", 96, time.Second)
	// Rewatching 0..10 must not increase its contribution.
	tracker.consume(mpvEvent{Event: "seek"}, now)
	send("time-pos", 0, 0)
	send("time-pos", 10, 10*time.Second)
	if got := watchedDuration(tracker.progress); got != 12 {
		t.Fatalf("coverage=%v ranges=%+v", got, tracker.progress.Ranges)
	}
	// Even if MPV misses a seek event, implausibly fast position jumps are ignored.
	send("time-pos", 99, time.Millisecond)
	if got := watchedDuration(tracker.progress); got != 12 {
		t.Fatal(got)
	}
	_, ended, normal := tracker.consume(mpvEvent{Event: "end-file", Reason: "quit"}, now)
	if !ended || normal {
		t.Fatal("quit was treated as normal completion")
	}
}

func TestPlaybackSaveHonorsManualOverrideSessionAndMediaVersion(t *testing.T) {
	a, path, p := playbackFixture(t)
	a.ToggleEpisodeWatched(42, 1)
	a.ToggleEpisodeWatched(42, 1) // explicit unwatch
	p.Position = 99
	p.Completed = true
	if err := a.savePlayback(42, 1, path, p, true); err != nil {
		t.Fatal(err)
	}
	list, _ := a.readLibrary()
	if slices.Contains(list[0].WatchedEps, 1) {
		t.Fatal("automatic update overrode manual unwatch")
	}
	p.SessionID = "old-session"
	if err := a.savePlayback(42, 1, path, p, true); err == nil {
		t.Fatal("stale session accepted")
	}
	p.SessionID = "session"
	if err := os.WriteFile(path, []byte("different media version"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.savePlayback(42, 1, path, p, true); err == nil {
		t.Fatal("changed media accepted")
	}
}

func TestContinueWatchingPreservesArchivedHistoryAndHide(t *testing.T) {
	a, path, p := playbackFixture(t)
	a.UnfollowLocal(42)
	items, err := a.GetContinueWatching()
	if err != nil || len(items) != 1 || !items[0].Available || items[0].Position != 30 {
		t.Fatalf("items=%+v error=%v", items, err)
	}
	if result := a.HideContinueWatching(42, 1); result != "Success" {
		t.Fatal(result)
	}
	p.Position = 40
	if err := a.savePlayback(42, 1, path, p, false); err != nil {
		t.Fatal(err)
	}
	items, _ = a.GetContinueWatching()
	if len(items) != 0 {
		t.Fatal("progress event resurrected a hidden card")
	}
	list, _ := a.readLibrary()
	if !list[0].Archived || list[0].Playback["1"].Position != 40 || list[0].LocalFiles["1"] != path {
		t.Fatal("hide erased history")
	}
}

func TestMPVStreamCompletionRequiresEOFAndCoverage(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		coverage     float64
		completed    bool
	}{
		{"complete", "eof", 95, true}, {"seek-to-end", "eof", 5, false}, {"manual-exit", "quit", 95, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, path, p := playbackFixture(t)
			p.Ranges = []ViewedRange{{0, tc.coverage}}
			events := []mpvEvent{property("path", path), property("duration", 100), property("time-pos", 99), {Event: "end-file", Reason: tc.reason}}
			var stream strings.Builder
			for _, event := range events {
				data, _ := json.Marshal(event)
				stream.Write(data)
				stream.WriteByte('\n')
			}
			_, completed := a.consumePlayerStream(context.Background(), strings.NewReader(stream.String()), 42, 1, path, p)
			list, _ := a.readLibrary()
			if completed != tc.completed || list[0].Playback["1"].Completed != tc.completed || slices.Contains(list[0].WatchedEps, 1) != tc.completed {
				t.Fatalf("completed=%v record=%+v", completed, list[0])
			}
		})
	}
}

func TestNextPlaybackCancellationAndDisabledSetting(t *testing.T) {
	a, path, _ := playbackFixture(t)
	a.markEpisodeDownloaded(42, 2, path)
	a.playerMu.Lock()
	a.scheduleNextLocked(42, 1)
	a.playerMu.Unlock()
	if a.GetNextPlayback() != nil {
		t.Fatal("auto next enabled by default")
	}
	cfg := a.configMgr.Snapshot()
	cfg.Player.AutoNext = true
	if err := a.configMgr.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	a.playerMu.Lock()
	a.scheduleNextLocked(42, 1)
	pending := a.nextPlayback
	a.playerMu.Unlock()
	if pending == nil || pending.Episode != 2 {
		t.Fatal("missing countdown")
	}
	a.CancelNextPlayback()
	if a.GetNextPlayback() != nil {
		t.Fatal("countdown not cancelled")
	}
	if result := a.startNextPlayback(pending); result != "Success" {
		t.Fatal(result)
	}
	a.playerMu.Lock()
	a.scheduleNextLocked(42, 1)
	pending = a.nextPlayback
	a.playerMu.Unlock()
	cfg = a.configMgr.Snapshot()
	cfg.Player.AutoNext = false
	if err := a.configMgr.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	if result := a.startNextPlayback(pending); result != "Success" {
		t.Fatal(result)
	}
	if len(a.playerSessions) != 0 || a.GetNextPlayback() != nil {
		t.Fatal("disabled auto next launched player")
	}
}

// A small PCM fixture lets the real player exercise IPC without a display,
// audio hardware, network, ffmpeg, or a user's MPV configuration.
func writePlaybackWAV(t *testing.T, path string, seconds int) {
	t.Helper()
	const rate = 8000
	size := seconds * rate * 2
	data := make([]byte, 44+size)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+size))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(size))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func waitPlayer(t *testing.T, a *App, key string) *playerSession {
	t.Helper()
	a.playerMu.Lock()
	session := a.playerSessions[key]
	a.playerMu.Unlock()
	if session == nil {
		t.Fatal("player session missing")
	}
	return session
}

func TestMPVLocalPlaybackIntegration(t *testing.T) {
	mpv, err := exec.LookPath("mpv")
	if err != nil {
		t.Skip("mpv not installed")
	}
	t.Run("normal completion", func(t *testing.T) {
		a, path, _ := playbackFixture(t)
		writePlaybackWAV(t, path, 30)
		a.configMgr.Data.Player.MPVPath = mpv
		a.configMgr.Data.Player.MPVArgs = "--no-config --vo=null --ao=null --speed=4 --really-quiet"
		if result := a.PlayLocalEpisode(42, 1); result != "Success" {
			t.Fatal(result)
		}
		session := waitPlayer(t, a, "42:1")
		select {
		case <-session.done:
		case <-time.After(15 * time.Second):
			a.shutdown(context.Background())
			t.Fatal("MPV did not finish")
		}
		list, _ := a.readLibrary()
		p := list[0].Playback["1"]
		if !p.Completed || !slices.Contains(list[0].WatchedEps, 1) {
			t.Fatalf("real MPV did not record completion: %+v", p)
		}
	})
	t.Run("quit saves position and next launch resumes", func(t *testing.T) {
		a, path, _ := playbackFixture(t)
		writePlaybackWAV(t, path, 30)
		a.configMgr.Data.Player.MPVPath = mpv
		a.configMgr.Data.Player.MPVArgs = "--no-config --vo=null --ao=null --really-quiet"
		for iteration := 0; iteration < 2; iteration++ {
			if result := a.PlayLocalEpisode(42, 1); result != "Success" {
				t.Fatal(result)
			}
			session := waitPlayer(t, a, "42:1")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			address := playerIPCAddress(filepath.Join(os.TempDir(), session.id))
			var position float64
			for ctx.Err() == nil {
				conn, err := dialPlayerIPC(ctx, address)
				if err != nil {
					time.Sleep(30 * time.Millisecond)
					continue
				}
				stopClose := context.AfterFunc(ctx, func() { conn.Close() })
				encoder, decoder := json.NewEncoder(conn), json.NewDecoder(conn)
				encoder.Encode(map[string]any{"command": []any{"get_property", "time-pos"}, "request_id": 99})
				var event mpvEvent
				for decoder.Decode(&event) == nil {
					if event.RequestID == 99 {
						json.Unmarshal(event.Data, &position)
						break
					}
				}
				if iteration == 0 && position < 2 || iteration == 1 && position < 1 {
					conn.Close()
					stopClose()
					time.Sleep(100 * time.Millisecond)
					continue
				}
				encoder.Encode(map[string]any{"command": []any{"quit"}})
				conn.Close()
				stopClose()
				break
			}
			cancel()
			select {
			case <-session.done:
			case <-time.After(5 * time.Second):
				a.shutdown(context.Background())
				t.Fatal("MPV quit did not flush")
			}
			list, _ := a.readLibrary()
			p := list[0].Playback["1"]
			if p.Completed || p.Position < 1.5 || len(list[0].WatchedEps) != 0 {
				t.Fatalf("iteration %d quit position=%v record=%+v", iteration, position, p)
			}
			if iteration == 1 && position < 1.5 {
				t.Fatal(fmt.Sprintf("resume restarted at %v", position))
			}
		}
	})
}
