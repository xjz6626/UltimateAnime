package bilibili

import (
	"bytes"
	"strings"
	"testing"
)

func TestASSConvertsModesColorAndEscapesCommentTags(t *testing.T) {
	comments := []Comment{
		{Time: 65.25, Mode: 1, FontSize: 25, Color: 0xff0000, Text: "滚动{\\pos(0,0)}"},
		{Time: 1, Mode: 5, FontSize: 25, Color: 0xffffff, Text: "顶部"},
		{Time: 2, Mode: 4, FontSize: 25, Color: 0xffffff, Text: "底部"},
		{Time: 3, Mode: 6, FontSize: 25, Color: 0xffffff, Text: "逆向"},
		{Time: 0, Mode: 8, Text: "code must not execute"},
	}
	var output bytes.Buffer
	count, err := WriteASS(&output, comments)
	if err != nil || count != 4 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	ass := output.String()
	for _, fragment := range []string{"PlayResX: 1920", "0:01:05.25,0:01:13.25", `\c&H0000FF&`, `\an8\pos(960,0)`, `\an2\pos(960,810)`, `滚动｛＼pos(0,0)｝`} {
		if !strings.Contains(ass, fragment) {
			t.Fatalf("ASS is missing %q: %s", fragment, ass)
		}
	}
	if strings.Contains(ass, "code must not execute") || strings.Index(ass, "顶部") > strings.Index(ass, "滚动") {
		t.Fatal("unsupported modes or event ordering are incorrect")
	}
	if comments[0].Time != 65.25 {
		t.Fatal("caller comments were reordered")
	}
}

func TestASSBoundsDensityAndAvoidsCatchup(t *testing.T) {
	var comments []Comment
	for i := 0; i < 100; i++ {
		comments = append(comments, Comment{Time: 1, Mode: 1, FontSize: 25, Text: "弹幕"})
	}
	var output bytes.Buffer
	count, err := WriteASS(&output, comments)
	if err != nil || count != 8 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	previous := lane{start: 0, end: 8, width: 40, speed: 245, mode: 1}
	if laneAvailable(previous, 1, 1000, 1) {
		t.Fatal("fast follower would catch the preceding comment")
	}
	if !laneAvailable(previous, 1, 245, 1) {
		t.Fatal("same speed follower with enough separation should fit")
	}
	if laneAvailable(previous, 1, 245, 6) {
		t.Fatal("opposite direction comments would collide")
	}
}

func TestXMLRejectsErrorPagesAndSkipsMalformedComments(t *testing.T) {
	if _, err := ParseXML([]byte(`<html><body>verification</body></html>`)); err == nil {
		t.Fatal("verification page accepted as danmaku")
	}
	comments, err := ParseXML([]byte(`<i><d p="bad">x</d><d p="1,1,25,16777215">ok</d><d p="-1,1,25,1">bad</d><d p="2,7,25,1">advanced</d></i>`))
	if err != nil || len(comments) != 1 || comments[0].Text != "ok" {
		t.Fatalf("comments=%+v err=%v", comments, err)
	}
	var output bytes.Buffer
	if _, err := WriteASS(&output, nil); err == nil || output.Len() != 0 {
		t.Fatal("empty danmaku must not create an apparently valid cache")
	}
}
