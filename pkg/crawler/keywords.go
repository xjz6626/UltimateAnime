package crawler

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	titleSeparators = regexp.MustCompile(`[：:～~·・/／|｜—–!?！？…「」『』《》【】\[\]（）()]+`)
	titlePart       = regexp.MustCompile(`(?i)(?:\b(?:part|cour)\s*\d+\b|第\s*[一二三四五六七八九十\d]+\s*部(?:分)?)`)
)

// Try full names first, then season-free names and substantial phrases from the
// title. Bound the list so a long title cannot generate unlimited network calls.
func episodeQueries(names []string) []string {
	queries := make([]string, 0, 8)
	seen := make(map[string]bool)
	add := func(value string) {
		value = strings.Join(strings.Fields(value), " ")
		key := strings.ToLower(value)
		if value != "" && !seen[key] && len(queries) < 8 {
			seen[key] = true
			queries = append(queries, value)
		}
	}
	for _, name := range names {
		add(name)
	}
	for _, name := range names {
		base := name
		for _, pattern := range seasonPatterns {
			base = pattern.ReplaceAllString(base, " ")
		}
		base = titlePart.ReplaceAllString(base, " ")
		base = strings.Trim(base, " \t-:：～~·・/／—–")
		if len([]rune(base)) >= 2 {
			add(base)
		}
		parts := titleSeparators.Split(base, -1)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if usefulTitleKeyword(part) {
				add(part)
			}
			for _, phrase := range strings.FieldsFunc(part, func(r rune) bool { return strings.ContainsRune("的之与和", r) }) {
				phrase = strings.TrimPrefix(strings.TrimSpace(phrase), "关于")
				if usefulTitleKeyword(phrase) {
					add(phrase)
				}
			}
		}
		// Space-separated names often contain the short name used by publishers.
		for _, word := range strings.Fields(base) {
			if len([]rune(word)) >= 4 && !strings.Contains(" season part final the animation ", " "+strings.ToLower(word)+" ") {
				add(word)
			}
		}
		// Chinese long titles may have no separators at all. A substantial prefix
		// also covers the shortened names commonly used in release titles.
		runes := []rune(base)
		if len(runes) > 12 {
			allHan := true
			for _, r := range runes {
				allHan = allHan && unicode.Is(unicode.Han, r)
			}
			if allHan {
				add(string(runes[:8]))
			}
		}
	}
	return queries
}

func usefulTitleKeyword(value string) bool {
	if value == "最终章" || value == "完结篇" {
		return false
	}
	var han, letters int
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			han++
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return han >= 2 || letters >= 4
}
