package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"UltimateAnime/pkg/bilibili"
	"UltimateAnime/pkg/config"
)

type fakeDanmakuSource struct {
	searches, seasons, downloads int
	season                       bilibili.Season
	commentsErr                  error
	lastCID                      int64
}

func (s *fakeDanmakuSource) SearchSeason(_ context.Context, names []string, year int) (int, error) {
	s.searches++
	if len(names) != 2 || names[0] != "测试 第二季" || year != 2026 {
		return 0, errors.New("missing followed title or year")
	}
	return s.season.ID, nil
}

func (s *fakeDanmakuSource) Season(_ context.Context, _ int) (bilibili.Season, error) {
	s.seasons++
	return s.season, nil
}

func (s *fakeDanmakuSource) Comments(_ context.Context, cid int64) ([]bilibili.Comment, error) {
	s.downloads++
	s.lastCID = cid
	return []bilibili.Comment{{Time: 1, Mode: 1, FontSize: 25, Color: 0xffffff, Text: "测试弹幕"}}, s.commentsErr
}

func danmakuTestApp(t *testing.T) (*App, string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	app := &App{
		configMgr: &config.Manager{ConfigPath: filepath.Join(dir, "config.json"), Data: config.NewDefaultConfig()},
		webEvents: newWebEventHub(),
	}
	if result := app.saveFollowedList([]FollowedItem{{SubjectID: 42, NameCN: "测试 第二季", Name: "Test Season 2", AirDate: "2026-01-01"}}); result != "Success" {
		t.Fatal(result)
	}
	return app, dir
}

func TestDanmakuDownloadsMatchingEpisodeAndLoadsAlongsideDialogue(t *testing.T) {
	app, dir := danmakuTestApp(t)
	source := &fakeDanmakuSource{season: bilibili.Season{ID: 2, Title: "测试 第二季", Episodes: []bilibili.Episode{{Title: "PV", CID: 999}, {Title: "1", CID: 101}, {Title: "2", CID: 102}}}}
	video := filepath.Join(dir, "视频 带空格.mkv")
	before, _ := os.ReadFile("followed.json")
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 2, video); err != nil {
		t.Fatal(err)
	}
	ass, err := os.ReadFile(danmakuPath(video))
	if err != nil || !strings.Contains(string(ass), "测试弹幕") || source.lastCID != 102 {
		t.Fatalf("ASS=%s CID=%d error=%v", ass, source.lastCID, err)
	}
	args := app.localDanmakuArgs(video)
	joined := strings.Join(args, "\n")
	if len(args) != 4 || args[0] != "--sub-file="+danmakuPath(video) || !strings.Contains(joined, "--sub-ass-override=no") || !strings.Contains(joined, "--secondary-sid=auto") {
		t.Fatal("MPV must receive an unsplit path and preserve a dialogue track", args)
	}
	app.prepareEpisodeDanmaku(context.Background(), 42, 2, video)
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 1, filepath.Join(dir, "next.mkv")); err != nil {
		t.Fatal(err)
	}
	if source.searches != 1 || source.seasons != 1 || source.downloads != 2 {
		t.Fatalf("cache not reused: %+v", source)
	}
	after, _ := os.ReadFile("followed.json")
	if string(before) != string(after) {
		t.Fatal("danmaku changed download/watch records")
	}
}

func TestDanmakuRefreshesSeasonWhenNextEpisodeWasNotCached(t *testing.T) {
	app, dir := danmakuTestApp(t)
	source := &fakeDanmakuSource{season: bilibili.Season{ID: 2, Title: "测试", Episodes: []bilibili.Episode{{Title: "1", CID: 101}}}}
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 1, filepath.Join(dir, "one.mp4")); err != nil {
		t.Fatal(err)
	}
	source.season.Episodes = append(source.season.Episodes, bilibili.Episode{Title: "2", CID: 102})
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 2, filepath.Join(dir, "two.mp4")); err != nil {
		t.Fatal(err)
	}
	if source.searches != 1 || source.seasons != 2 || source.lastCID != 102 {
		t.Fatalf("next episode did not refresh metadata: %+v", source)
	}
}

func TestDanmakuFailureLeavesVideoAndNoPartialASS(t *testing.T) {
	app, dir := danmakuTestApp(t)
	video := filepath.Join(dir, "episode.mp4")
	if err := os.WriteFile(video, []byte("downloaded-video"), 0600); err != nil {
		t.Fatal(err)
	}
	source := &fakeDanmakuSource{season: bilibili.Season{ID: 2, Title: "测试", Episodes: []bilibili.Episode{{Title: "1", CID: 101}}}, commentsErr: errors.New("Bilibili unavailable")}
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 1, video); err == nil {
		t.Fatal("expected danmaku error")
	}
	data, _ := os.ReadFile(video)
	if string(data) != "downloaded-video" {
		t.Fatal("video was changed by failed danmaku download")
	}
	if _, err := os.Stat(danmakuPath(video)); !os.IsNotExist(err) {
		t.Fatal("failure left a partial ASS", err)
	}
	if args := app.localDanmakuArgs(video); len(args) != 0 {
		t.Fatal(args)
	}
	source.commentsErr = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.downloadEpisodeDanmaku(ctx, source, 42, 1, video); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDanmakuSwitchAndDeleteEpisodeSidecar(t *testing.T) {
	app, dir := danmakuTestApp(t)
	video := filepath.Join(dir, "episode.mkv")
	for _, path := range []string{video, danmakuPath(video)} {
		if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	app.markEpisodeDownloaded(42, 1, video)
	cfg := app.configMgr.Snapshot()
	cfg.Danmaku.Enabled = false
	if err := app.configMgr.Replace(cfg); err != nil {
		t.Fatal(err)
	}
	if args := app.localDanmakuArgs(video); len(args) != 0 {
		t.Fatal("disabled danmaku was still passed to MPV", args)
	}
	if result := app.DeleteEpisodeData(42, 1); result != "Success" {
		t.Fatal(result)
	}
	for _, path := range []string{video, danmakuPath(video)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("deleted episode left its sidecar", path, err)
		}
	}
}

func TestManualDanmakuChoiceBypassesAutomaticMatching(t *testing.T) {
	app, dir := danmakuTestApp(t)
	path, err := app.danmakuChoicePath(42, 2)
	if err != nil {
		t.Fatal(err)
	}
	choice := DanmakuChoice{Kind: "video", ID: "BV13x41117TL", CID: 777, Title: "用户选择的视频", PartTitle: "P3"}
	if err := app.saveJSONCache(path, choice); err != nil {
		t.Fatal(err)
	}
	source := &fakeDanmakuSource{}
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 2, filepath.Join(dir, "manual.mkv")); err != nil {
		t.Fatal(err)
	}
	if source.searches != 0 || source.seasons != 0 || source.lastCID != 777 {
		t.Fatalf("manual selection was ignored: %+v", source)
	}
	loaded, err := app.GetEpisodeDanmakuSource(42, 2)
	if err != nil || loaded.Choice != choice {
		t.Fatal(loaded, err)
	}
}

func TestSkipDanmakuRemovesOnlyOldCommentSidecar(t *testing.T) {
	app, dir := danmakuTestApp(t)
	video := filepath.Join(dir, "skip.mkv")
	for _, path := range []string{video, danmakuPath(video)} {
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	app.markEpisodeDownloaded(42, 1, video)
	if result := app.SaveEpisodeDanmakuSource(42, 1, DanmakuChoice{Kind: "skip", CID: 999}); result != "Success" {
		t.Fatal(result)
	}
	choice, err := app.loadDanmakuChoice(42, 1)
	if err != nil || choice.Kind != "skip" || choice.CID != 0 {
		t.Fatal(choice, err)
	}
	source := &fakeDanmakuSource{}
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 1, video); err != nil || source.downloads != 0 {
		t.Fatal("skipped comments were downloaded", err)
	}
	if _, err := os.Stat(video); err != nil {
		t.Fatal("video was removed", err)
	}
	if _, err := os.Stat(danmakuPath(video)); !os.IsNotExist(err) {
		t.Fatal("disabled sidecar retained", err)
	}
}

type changingDanmakuSource struct {
	*fakeDanmakuSource
	change func()
}

func (s changingDanmakuSource) Comments(ctx context.Context, cid int64) ([]bilibili.Comment, error) {
	s.change()
	return s.fakeDanmakuSource.Comments(ctx, cid)
}

func TestChangingDanmakuSourceDiscardsPreviousInFlightComments(t *testing.T) {
	app, dir := danmakuTestApp(t)
	path, _ := app.danmakuChoicePath(42, 1)
	if err := app.saveJSONCache(path, DanmakuChoice{Kind: "video", ID: "BV13x41117TL", CID: 777}); err != nil {
		t.Fatal(err)
	}
	source := changingDanmakuSource{fakeDanmakuSource: &fakeDanmakuSource{}, change: func() {
		if result := app.SaveEpisodeDanmakuSource(42, 1, DanmakuChoice{Kind: "skip"}); result != "Success" {
			t.Fatal(result)
		}
	}}
	video := filepath.Join(dir, "changed.mkv")
	if err := app.downloadEpisodeDanmaku(context.Background(), source, 42, 1, video); err == nil {
		t.Fatal("stale source was published")
	}
	if _, err := os.Stat(danmakuPath(video)); !os.IsNotExist(err) {
		t.Fatal("previous source ASS retained", err)
	}
}
