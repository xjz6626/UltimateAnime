package bilibili

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Comment struct {
	Time     float64
	Mode     int
	FontSize int
	Color    uint32
	Text     string
}

// ParseXML ignores malformed individual entries, but rejects non-danmaku/error
// documents so an HTTP 200 verification page cannot become a cached empty ASS.
func ParseXML(body []byte) ([]Comment, error) {
	var document struct {
		XMLName xml.Name `xml:"i"`
		Entries []struct {
			Params string `xml:"p,attr"`
			Text   string `xml:",chardata"`
		} `xml:"d"`
	}
	if err := xml.Unmarshal(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")), &document); err != nil {
		return nil, fmt.Errorf("B 站弹幕 XML 无效")
	}
	comments := make([]Comment, 0, len(document.Entries))
	for _, entry := range document.Entries {
		fields := strings.Split(entry.Params, ",")
		if len(fields) < 4 {
			continue
		}
		at, timeErr := strconv.ParseFloat(fields[0], 64)
		mode, modeErr := strconv.Atoi(fields[1])
		font, fontErr := strconv.Atoi(fields[2])
		color, colorErr := strconv.ParseUint(fields[3], 10, 24)
		if timeErr != nil || modeErr != nil || fontErr != nil || colorErr != nil || !validTime(at) {
			continue
		}
		text := cleanText(entry.Text)
		if text == "" || len([]rune(text)) > 400 || mode < 1 || mode > 6 {
			continue
		}
		comments = append(comments, Comment{Time: at, Mode: mode, FontSize: font, Color: uint32(color), Text: text})
	}
	return comments, nil
}

func validTime(at float64) bool {
	return !math.IsNaN(at) && !math.IsInf(at, 0) && at >= 0 && at <= 24*60*60
}

func cleanText(text string) string {
	// ASS has executable formatting tags; display user comments as literal text.
	text = strings.NewReplacer("\\", "＼", "{", "｛", "}", "｝").Replace(text)
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

type lane struct {
	start, end, width, speed float64
	mode                     int
}

// WriteASS lays out ordinary, reverse, top and bottom comments with bounded
// density. The bottom quarter is left free for the video's dialogue subtitles.
func WriteASS(writer io.Writer, comments []Comment) (int, error) {
	const width, height, rowHeight = 1920, 1080, 64
	const movingDuration, stillDuration = 8.0, 4.0
	ordered := append([]Comment(nil), comments...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time < ordered[j].Time })
	var output strings.Builder
	output.WriteString(`[Script Info]
Title: Bilibili Danmaku
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
WrapStyle: 2
ScaledBorderAndShadow: yes
YCbCr Matrix: None

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Danmaku,sans-serif,40,&H33FFFFFF,&H33FFFFFF,&H33000000,&HFF000000,0,0,0,0,100,100,0,0,1,1.5,0,7,0,0,0,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`)
	moving := make([]lane, height*3/4/rowHeight-4)
	top, bottom := make([]lane, 2), make([]lane, 2)
	count := 0
	for _, comment := range ordered {
		if !validTime(comment.Time) || comment.Mode < 1 || comment.Mode > 6 {
			continue
		}
		text := cleanText(comment.Text)
		if text == "" || len([]rune(text)) > 400 {
			continue
		}
		font := math.Max(28, math.Min(56, float64(comment.FontSize)*1.6))
		textWidth := 0.0
		for _, r := range text {
			if r < 128 {
				textWidth += font * 0.6
			} else {
				textWidth += font
			}
		}
		textWidth = math.Ceil(textWidth) + 8
		duration, position := movingDuration, ""
		if comment.Mode == 4 || comment.Mode == 5 {
			duration = stillDuration
			rows := top
			if comment.Mode == 4 {
				rows = bottom
			}
			for i := range rows {
				if rows[i].end > comment.Time {
					continue
				}
				rows[i].end = comment.Time + duration
				y, align := i*rowHeight, 8
				if comment.Mode == 4 {
					y, align = height*3/4-i*rowHeight, 2
				}
				position = fmt.Sprintf("\\an%d\\pos(%d,%d)", align, width/2, y)
				break
			}
		} else {
			mode := comment.Mode
			if mode <= 3 {
				mode = 1
			}
			speed := (width + textWidth) / duration
			for i, previous := range moving {
				if !laneAvailable(previous, comment.Time, speed, mode) {
					continue
				}
				moving[i] = lane{start: comment.Time, end: comment.Time + duration, width: textWidth, speed: speed, mode: mode}
				y := (i + 2) * rowHeight
				from, to := width, -int(textWidth)
				if mode == 6 {
					from, to = to, from
				}
				position = fmt.Sprintf("\\an7\\move(%d,%d,%d,%d)", from, y, to, y)
				break
			}
		}
		if position == "" {
			continue
		}
		color := comment.Color
		bgr := ((color & 0xff) << 16) | (color & 0xff00) | ((color >> 16) & 0xff)
		fmt.Fprintf(&output, "Dialogue: 0,%s,%s,Danmaku,,0,0,0,,{\\fs%.0f\\c&H%06X&%s}%s\n",
			assTime(comment.Time), assTime(comment.Time+duration), font, bgr, position, text)
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("没有可转换的普通弹幕")
	}
	_, err := io.WriteString(writer, output.String())
	return count, err
}

func laneAvailable(previous lane, at, speed float64, mode int) bool {
	if previous.end <= at {
		return true
	}
	if previous.mode != mode {
		return false
	}
	gap := previous.speed*(at-previous.start) - previous.width - 24
	if gap < 0 {
		return false
	}
	// A faster follower must not catch the earlier comment before it exits.
	return speed <= previous.speed || gap/(speed-previous.speed) >= previous.end-at
}

func assTime(seconds float64) string {
	centiseconds := int(math.Round(seconds * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", centiseconds/360000, centiseconds/6000%60, centiseconds/100%60, centiseconds%100)
}
