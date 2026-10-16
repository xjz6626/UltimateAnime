package bilibili

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	htmlTags      = regexp.MustCompile(`<[^>]*>`)
	regionSuffix  = regexp.MustCompile(`[（(](?:仅限|僅限)[^）)]*(?:地区|地區)[）)]`)
	chineseSeason = regexp.MustCompile(`第\s*([一二三四五六七八九十百零〇两兩0-9]+)\s*季`)
	englishSeason = regexp.MustCompile(`(?i)(?:season\s*(\d+)|(\d+)(?:st|nd|rd|th)\s+season)`)
	episodeNumber = regexp.MustCompile(`^(?:第\s*)?(\d+(?:\.\d+)?)(?:\s*[话話集])?$`)
)

func normalizedTitle(title string) string {
	title = html.UnescapeString(htmlTags.ReplaceAllString(title, ""))
	title = regionSuffix.ReplaceAllString(title, "")
	title = chineseSeason.ReplaceAllStringFunc(title, func(value string) string {
		match := chineseSeason.FindStringSubmatch(value)
		number, err := strconv.Atoi(match[1])
		if err != nil {
			number, _ = chineseNumber(match[1])
		}
		if number == 0 {
			return value
		}
		return fmt.Sprintf("season%d", number)
	})
	title = englishSeason.ReplaceAllStringFunc(title, func(value string) string {
		match := englishSeason.FindStringSubmatch(value)
		return "season" + match[1] + match[2]
	})
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, title)
}

func chineseNumber(value string) (int, bool) {
	digits := map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '兩': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	number, digit := 0, 0
	for _, r := range value {
		if r == '十' || r == '百' {
			unit := 10
			if r == '百' {
				unit = 100
			}
			if digit == 0 {
				digit = 1
			}
			number, digit = number+digit*unit, 0
		} else if n, ok := digits[r]; ok {
			digit = n
		} else {
			return 0, false
		}
	}
	return number + digit, true
}

func chooseSeason(results []searchResult, names []string, year int) (int, error) {
	wanted := make(map[string]bool)
	for _, name := range names {
		if key := normalizedTitle(name); key != "" {
			wanted[key] = true
		}
	}
	bestID, bestScore, ambiguous := 0, 0, false
	for _, result := range results {
		if result.ID <= 0 || (!wanted[normalizedTitle(result.Title)] && !wanted[normalizedTitle(result.Original)]) {
			continue
		}
		score := 100
		if year > 0 && result.PubTime > 0 {
			resultYear := time.Unix(result.PubTime, 0).UTC().Year()
			if resultYear == year {
				score += 20
			} else {
				// Identical titles can also belong to remakes in another year.
				continue
			}
		}
		if !regionSuffix.MatchString(result.Title) {
			score++
		}
		if score > bestScore {
			bestID, bestScore, ambiguous = result.ID, score, false
		} else if score == bestScore && bestID != result.ID {
			ambiguous = true
		}
	}
	if bestID == 0 {
		return 0, fmt.Errorf("B 站未找到名称及季度相符的番剧")
	}
	if ambiguous {
		return 0, fmt.Errorf("B 站有多个同名番剧，无法可靠匹配，已跳过弹幕")
	}
	return bestID, nil
}

func (s Season) Episode(sort float64) (Episode, error) {
	if sort <= 0 || math.IsNaN(sort) || math.IsInf(sort, 0) {
		return Episode{}, fmt.Errorf("无效的集数")
	}
	var found Episode
	for _, episode := range s.Episodes {
		match := episodeNumber.FindStringSubmatch(strings.TrimSpace(episode.Title))
		if match == nil {
			continue // Never substitute a trailer or infer numbering from its index.
		}
		number, _ := strconv.ParseFloat(match[1], 64)
		if number != sort || episode.CID <= 0 {
			continue
		}
		if found.CID > 0 && found.CID != episode.CID {
			return Episode{}, fmt.Errorf("B 站第 %g 集存在多个版本，已跳过弹幕", sort)
		}
		found = episode
	}
	if found.CID == 0 {
		return Episode{}, fmt.Errorf("B 站番剧 %s 未找到第 %g 集正片", s.Title, sort)
	}
	return found, nil
}
