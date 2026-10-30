package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pathAttrRegex   = regexp.MustCompile(`(?i)\b(?:path|file|src)\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	fromAttrRegex   = regexp.MustCompile(`(?i)\bfrom\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	toAttrRegex     = regexp.MustCompile(`(?i)\bto\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	symbolAttrRegex = regexp.MustCompile(`(?i)\b(?:symbols?|def|func|struct|method)\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	linesAttrRegex  = regexp.MustCompile(`(?i)\blines\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	afterAttrRegex  = regexp.MustCompile(`(?i)\bafter\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)
	beforeAttrRegex = regexp.MustCompile(`(?i)\bbefore\s*=\s*["'\x{201c}\x{2018}]([^"'""\x{201d}\x{2019}\r\n>]+)["'\x{201d}\x{2019}]`)

	tagStartRegex     = regexp.MustCompile(`(?m)^\s*\x3c(fi` + `le|rep` + `lace|del` + `ete)\b`)
	searchRegex       = regexp.MustCompile(`(?s)\x3c\s*sea` + `rch\s*>(.*?)\x3c/\s*sea` + `rch(?:\s+[^>]*)?>`)
	withRegex         = regexp.MustCompile(`(?s)\x3c\s*wi` + `th\s*>(.*?)\x3c/\s*wi` + `th(?:\s+[^>]*)?>`)
	fromTagRegex      = regexp.MustCompile(`(?s)\x3c\s*fr` + `om\s*>(.*?)\x3c/\s*fr` + `om(?:\s+[^>]*)?>`)
	toTagRegex        = regexp.MustCompile(`(?s)\x3c\s*t` + `o\s*>(.*?)\x3c/\s*t` + `o(?:\s+[^>]*)?>`)
	symbolTagRegex    = regexp.MustCompile(`(?s)\x3c\s*sym` + `bol\s*>(.*?)\x3c/\s*sym` + `bol(?:\s+[^>]*)?>`)
	topLevelDirective = regexp.MustCompile(`(?m)^\s*\x3c(fi` + `le|rep` + `lace|del` + `ete)\b[^>]*\b(?:path|file|src)\s*=`)

	leadingTagRegex    = regexp.MustCompile(`^(?i)\s*\x3c(?:fi\s*le|rep\s*lace|with|sea\s*rch|content)\b[^>]*>\s*`)
	trailingTagRegex   = regexp.MustCompile(`(?i)\s*\x3c/(?:fi\s*le|rep\s*lace|with|sea\s*rch|content)(?:\s+[^>]*)?>\s*$`)
	linePrefixNumRegex = regexp.MustCompile(`(?m)^\s*\d+\s*\|\s?`)
)

func cleanContent(s string) string {
	if strings.Contains(s, "<") || strings.Contains(s, ">") {
		replacer := strings.NewReplacer(
			"<file", "<file",
			"</file", "</file",
			"<replace", "<replace",
			"</replace", "</replace",
			"<search", "<search",
			"</search", "</search",
			"<with", "<with",
			"</with", "</with",
			"<delete", "<delete",
			"</delete", "</delete",
			"<from", "<from",
			"</from", "</from",
			"<to", "<to",
			"</to", "</to",
			"<symbol", "<symbol",
			"</symbol", "</symbol",
		)
		s = replacer.Replace(s)
	}
	return s
}

func trimTagNewlines(s string) string {
	if strings.HasPrefix(s, "\r\n") {
		s = s[2:]
	} else if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}

	if strings.HasSuffix(s, "\r\n") {
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	return s
}

func stripCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") && !strings.HasPrefix(trimmed, "~~~") {
		return s
	}
	lines := strings.Split(s, "\n")
	startIdx := -1
	endIdx := -1

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
				startIdx = i
			}
			break
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
				endIdx = i
			}
			break
		}
	}

	if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
		return strings.Join(lines[startIdx+1:endIdx], "\n")
	}
	if startIdx != -1 && endIdx == -1 {
		return strings.Join(lines[startIdx+1:], "\n")
	}
	return s
}

func stripLineNumbers(s string) string {
	lines := strings.Split(s, "\n")
	matchCount := 0
	nonEmptyCount := 0

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		nonEmptyCount++
		if linePrefixNumRegex.MatchString(l) {
			matchCount++
		}
	}

	if nonEmptyCount > 0 && float64(matchCount)/float64(nonEmptyCount) >= 0.5 {
		for i, l := range lines {
			lines[i] = linePrefixNumRegex.ReplaceAllString(l, "")
		}
		return strings.Join(lines, "\n")
	}
	return s
}

func stripResidualTags(s string) string {
	for {
		changed := false
		trimmed := strings.TrimSpace(s)
		if loc := leadingTagRegex.FindStringIndex(trimmed); loc != nil && loc[0] == 0 {
			trimmed = trimmed[loc[1]:]
			s = trimmed
			changed = true
		}
		trimmed = strings.TrimSpace(s)
		if loc := trailingTagRegex.FindStringIndex(trimmed); loc != nil && loc[1] == len(trimmed) {
			trimmed = trimmed[:loc[0]]
			s = trimmed
			changed = true
		}
		if !changed {
			break
		}
	}
	return s
}

func sanitizeContent(s string) string {
	s = trimTagNewlines(s)
	s = stripCodeFences(s)
	s = stripLineNumbers(s)
	s = stripResidualTags(s)
	return s
}

func lineAtOffset(s string, offset int) int {
	if offset > len(s) {
		offset = len(s)
	}
	return strings.Count(s[:offset], "\n") + 1
}

func parsePatch(content string) ([]Action, []string) {
	var actions []Action
	var errs []string

	pos := 0
	for {
		loc := tagStartRegex.FindStringIndex(content[pos:])
		if loc == nil {
			break
		}

		startIdx := pos + loc[0]
		lineNo := lineAtOffset(content, startIdx)

		openTagEnd := strings.Index(content[startIdx:], ">")
		if openTagEnd == -1 {
			errs = append(errs, fmt.Sprintf("Line %d: Malformed unclosed tag header", lineNo))
			break
		}
		openTagEnd += startIdx + 1
		header := content[startIdx:openTagEnd]

		trimmedHeader := strings.TrimSpace(header)
		isSelfClosing := strings.HasSuffix(trimmedHeader, "/>")

		pathMatch := pathAttrRegex.FindStringSubmatch(header)
		if len(pathMatch) < 2 {
			pos = openTagEnd
			continue
		}
		targetPath := filepath.Clean(strings.TrimSpace(pathMatch[1]))

		switch {
		case strings.HasPrefix(strings.ToLower(trimmedHeader), "\x3c"+"del"+"ete"):
			actions = append(actions, Action{
				Type: ActionDelete,
				Path: targetPath,
				Line: lineNo,
			})
			pos = openTagEnd

		case strings.HasPrefix(strings.ToLower(trimmedHeader), "\x3c"+"fi"+"le"):
			if isSelfClosing {
				pos = openTagEnd
				continue
			}

			closeRe := regexp.MustCompile(`(?m)^\s*\x3c/\s*fi` + `le(?:\s+[^>]*)?>`)
			m := closeRe.FindStringIndex(content[openTagEnd:])
			if m == nil {
				errs = append(errs, fmt.Sprintf("Line %d: Unclosed \x3c"+"fi"+"le path=\"%s\"> (no matching \x3c/"+"fi"+"le> found)", lineNo, targetPath))
				pos = openTagEnd
				continue
			}

			closeIdx := m[0]
			body := content[openTagEnd : openTagEnd+closeIdx]

			if topLevelDirective.MatchString(body) {
				trimmedBody := strings.TrimSpace(body)
				if loc := leadingTagRegex.FindStringIndex(trimmedBody); loc != nil && loc[0] == 0 {
					body = trimmedBody[loc[1]:]
				}
				if topLevelDirective.MatchString(body) {
					errs = append(errs, fmt.Sprintf("Line %d: Corrupted patch! \x3c"+"fi"+"le path=\"%s\"> appears to swallow another top-level directive before \x3c/"+"fi"+"le>.", lineNo, targetPath))
					pos = openTagEnd + m[1]
					continue
				}
			}

			body = sanitizeContent(body)

			actions = append(actions, Action{
				Type: ActionFile,
				Path: targetPath,
				Body: body,
				Line: lineNo,
			})
			pos = openTagEnd + m[1]

		case strings.HasPrefix(strings.ToLower(trimmedHeader), "\x3c"+"rep"+"lace"):
			if isSelfClosing {
				pos = openTagEnd
				continue
			}

			closeRe := regexp.MustCompile(`(?m)^\s*\x3c/\s*rep` + `lace(?:\s+[^>]*)?>`)
			m := closeRe.FindStringIndex(content[openTagEnd:])
			if m == nil {
				errs = append(errs, fmt.Sprintf("Line %d: Unclosed \x3c"+"rep"+"lace path=\"%s\"> (no matching \x3c/"+"rep"+"lace> found)", lineNo, targetPath))
				pos = openTagEnd
				continue
			}

			inner := content[openTagEnd : openTagEnd+m[0]]

			fromVal := ""
			if fromMatch := fromAttrRegex.FindStringSubmatch(header); len(fromMatch) >= 2 {
				fromVal = strings.TrimSpace(fromMatch[1])
			} else if ftMatch := fromTagRegex.FindStringSubmatch(inner); len(ftMatch) >= 2 {
				fromVal = strings.TrimSpace(ftMatch[1])
			}

			toVal := ""
			if toMatch := toAttrRegex.FindStringSubmatch(header); len(toMatch) >= 2 {
				toVal = strings.TrimSpace(toMatch[1])
			} else if ttMatch := toTagRegex.FindStringSubmatch(inner); len(ttMatch) >= 2 {
				toVal = strings.TrimSpace(ttMatch[1])
			}

			symbolVal := ""
			if symMatch := symbolAttrRegex.FindStringSubmatch(header); len(symMatch) >= 2 {
				symbolVal = strings.TrimSpace(symMatch[1])
			} else if stMatch := symbolTagRegex.FindStringSubmatch(inner); len(stMatch) >= 2 {
				symbolVal = strings.TrimSpace(stMatch[1])
			}

			linesVal := ""
			if linesMatch := linesAttrRegex.FindStringSubmatch(header); len(linesMatch) >= 2 {
				linesVal = strings.TrimSpace(linesMatch[1])
			}

			afterVal := ""
			if afterMatch := afterAttrRegex.FindStringSubmatch(header); len(afterMatch) >= 2 {
				afterVal = strings.TrimSpace(afterMatch[1])
			}

			beforeVal := ""
			if beforeMatch := beforeAttrRegex.FindStringSubmatch(header); len(beforeMatch) >= 2 {
				beforeVal = strings.TrimSpace(beforeMatch[1])
			}

			searchMatch := searchRegex.FindStringSubmatch(inner)
			withMatch := withRegex.FindStringSubmatch(inner)

			searchStr := ""
			withStr := ""

			hasTargetDirective := fromVal != "" || symbolVal != "" || linesVal != "" || afterVal != "" || beforeVal != ""

			if len(withMatch) >= 2 {
				withStr = sanitizeContent(withMatch[1])
			} else if hasTargetDirective {
				withStr = sanitizeContent(inner)
			} else {
				errs = append(errs, fmt.Sprintf("Line %d: \x3c"+"rep"+"lace path=\"%s\"> is missing \x3c"+"wi"+"th> block", lineNo, targetPath))
				pos = openTagEnd + m[1]
				continue
			}

			if len(searchMatch) >= 2 {
				searchStr = sanitizeContent(searchMatch[1])
			} else if !hasTargetDirective {
				if topLevelDirective.MatchString(inner) {
					errs = append(errs, fmt.Sprintf("Line %d: Corrupted patch! Unclosed directive detected inside \x3c"+"rep"+"lace path=\"%s\">", lineNo, targetPath))
				} else {
					errs = append(errs, fmt.Sprintf("Line %d: \x3c"+"rep"+"lace path=\"%s\"> is missing \x3c"+"sea"+"rch> block (or targeted from/to, symbol, or lines directive)", lineNo, targetPath))
				}
				pos = openTagEnd + m[1]
				continue
			}

			actions = append(actions, Action{
				Type:   ActionReplace,
				Path:   targetPath,
				Search: searchStr,
				With:   withStr,
				From:   fromVal,
				To:     toVal,
				Symbol: symbolVal,
				Lines:  linesVal,
				After:  afterVal,
				Before: beforeVal,
				Line:   lineNo,
			})
			pos = openTagEnd + m[1]
		}
	}

	return actions, errs
}
