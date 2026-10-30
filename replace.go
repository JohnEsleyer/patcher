package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func applyReplace(origStr string, act Action) (string, error) {
	if act.Lines != "" {
		return replaceLines(origStr, act.Lines, act.With)
	}

	if act.Symbol != "" {
		return replaceSymbol(origStr, act.Path, act.Symbol, act.With)
	}

	if act.From != "" {
		return replaceFromTo(origStr, act.From, act.To, act.With)
	}

	if act.After != "" {
		return insertAfter(origStr, act.After, act.With)
	}

	if act.Before != "" {
		return insertBefore(origStr, act.Before, act.With)
	}

	if hasEllipsis(act.Search) {
		return replaceWithEllipsis(origStr, act.Search, act.With)
	}

	newStr, ok := performReplace(origStr, act.Search, act.With)
	if !ok {
		hint := findMismatchHint(origStr, act.Search)
		errDesc := "Search target not found"
		if hint != "" {
			errDesc += fmt.Sprintf("\n     Mismatch hint %s", hint)
		}
		return "", fmt.Errorf("%s", errDesc)
	}
	return newStr, nil
}

func performReplace(origStr, search, with string) (string, bool) {
	if strings.Contains(origStr, search) {
		return strings.Replace(origStr, search, with, 1), true
	}

	normalizedOrig := strings.ReplaceAll(origStr, "\r\n", "\n")
	normalizedSearch := strings.ReplaceAll(search, "\r\n", "\n")
	normalizedWith := strings.ReplaceAll(with, "\r\n", "\n")

	if strings.Contains(normalizedOrig, normalizedSearch) {
		res := strings.Replace(normalizedOrig, normalizedSearch, normalizedWith, 1)
		if strings.Contains(origStr, "\r\n") {
			res = strings.ReplaceAll(res, "\n", "\r\n")
		}
		return res, true
	}

	origLines := strings.Split(normalizedOrig, "\n")
	searchLines := strings.Split(normalizedSearch, "\n")

	if len(searchLines) <= len(origLines) {
		matchIdx := -1
		for i := 0; i <= len(origLines)-len(searchLines); i++ {
			matched := true
			for j := 0; j < len(searchLines); j++ {
				if strings.TrimRight(origLines[i+j], " \t\r") != strings.TrimRight(searchLines[j], " \t\r") {
					matched = false
					break
				}
			}
			if matched && matchIdx == -1 {
				matchIdx = i
				break
			}
		}
		if matchIdx != -1 {
			withLines := strings.Split(normalizedWith, "\n")
			newLines := make([]string, 0, len(origLines)-len(searchLines)+len(withLines))
			newLines = append(newLines, origLines[:matchIdx]...)
			newLines = append(newLines, withLines...)
			newLines = append(newLines, origLines[matchIdx+len(searchLines):]...)
			res := strings.Join(newLines, "\n")
			if strings.Contains(origStr, "\r\n") {
				res = strings.ReplaceAll(res, "\n", "\r\n")
			}
			return res, true
		}
	}

	if len(searchLines) <= len(origLines) {
		matchIdx := -1
		for i := 0; i <= len(origLines)-len(searchLines); i++ {
			matched := true
			for j := 0; j < len(searchLines); j++ {
				if strings.TrimSpace(origLines[i+j]) != strings.TrimSpace(searchLines[j]) {
					matched = false
					break
				}
			}
			if matched && matchIdx == -1 {
				matchIdx = i
				break
			}
		}
		if matchIdx != -1 {
			withLines := strings.Split(normalizedWith, "\n")
			newLines := make([]string, 0, len(origLines)-len(searchLines)+len(withLines))
			newLines = append(newLines, origLines[:matchIdx]...)
			newLines = append(newLines, withLines...)
			newLines = append(newLines, origLines[matchIdx+len(searchLines):]...)
			res := strings.Join(newLines, "\n")
			if strings.Contains(origStr, "\r\n") {
				res = strings.ReplaceAll(res, "\n", "\r\n")
			}
			return res, true
		}
	}

	return "", false
}

func replaceLines(origStr, linesSpec, with string) (string, error) {
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")
	totalLines := len(origLines)

	startLine := 1
	endLine := totalLines

	parts := strings.Split(linesSpec, "-")
	if len(parts) == 1 {
		parts = strings.Split(linesSpec, ":")
	}

	if len(parts) >= 2 {
		sStr := strings.TrimSpace(parts[0])
		eStr := strings.TrimSpace(parts[1])
		if v, err := strconv.Atoi(sStr); err == nil && v >= 1 {
			startLine = v
		}
		if v, err := strconv.Atoi(eStr); err == nil && v >= 1 {
			endLine = v
		}
	} else if len(parts) == 1 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && v >= 1 {
			startLine = v
			endLine = v
		}
	}

	return replaceLineSpan(origStr, startLine, endLine, with)
}

func replaceFromTo(origStr, fromPat, toPat, with string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")

	startIdx := -1
	endIdx := -1

	fromLower := strings.ToLower(fromPat)
	fromRe, fromErr := regexp.Compile("(?i)" + fromPat)

	for i, l := range origLines {
		matched := false
		if fromErr == nil {
			matched = fromRe.MatchString(l)
		} else {
			matched = strings.Contains(strings.ToLower(l), fromLower)
		}
		if matched {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return "", fmt.Errorf("start pattern %q not found", fromPat)
	}

	if toPat == "" {
		endIdx = startIdx
	} else {
		toLower := strings.ToLower(toPat)
		toRe, toErr := regexp.Compile("(?i)" + toPat)
		for j := startIdx; j < len(origLines); j++ {
			matched := false
			if toErr == nil {
				matched = toRe.MatchString(origLines[j])
			} else {
				matched = strings.Contains(strings.ToLower(origLines[j]), toLower)
			}
			if matched {
				endIdx = j
				break
			}
		}
		if endIdx == -1 {
			return "", fmt.Errorf("end pattern %q not found after line %d", toPat, startIdx+1)
		}
	}

	withLines := strings.Split(strings.ReplaceAll(with, "\r\n", "\n"), "\n")
	newLines := make([]string, 0, startIdx+len(withLines)+(len(origLines)-(endIdx+1)))
	newLines = append(newLines, origLines[:startIdx]...)
	newLines = append(newLines, withLines...)
	if endIdx+1 < len(origLines) {
		newLines = append(newLines, origLines[endIdx+1:]...)
	}

	res := strings.Join(newLines, "\n")
	if isCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
}

func insertAfter(origStr, afterPat, with string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")

	afterLower := strings.ToLower(afterPat)
	matchIdx := -1
	for i, l := range origLines {
		if strings.Contains(strings.ToLower(l), afterLower) {
			matchIdx = i
			break
		}
	}

	if matchIdx == -1 {
		return "", fmt.Errorf("anchor 'after' %q not found", afterPat)
	}

	withLines := strings.Split(strings.ReplaceAll(with, "\r\n", "\n"), "\n")
	newLines := make([]string, 0, len(origLines)+len(withLines))
	newLines = append(newLines, origLines[:matchIdx+1]...)
	newLines = append(newLines, withLines...)
	if matchIdx+1 < len(origLines) {
		newLines = append(newLines, origLines[matchIdx+1:]...)
	}

	res := strings.Join(newLines, "\n")
	if isCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
}

func insertBefore(origStr, beforePat, with string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")

	beforeLower := strings.ToLower(beforePat)
	matchIdx := -1
	for i, l := range origLines {
		if strings.Contains(strings.ToLower(l), beforeLower) {
			matchIdx = i
			break
		}
	}

	if matchIdx == -1 {
		return "", fmt.Errorf("anchor 'before' %q not found", beforePat)
	}

	withLines := strings.Split(strings.ReplaceAll(with, "\r\n", "\n"), "\n")
	newLines := make([]string, 0, len(origLines)+len(withLines))
	newLines = append(newLines, origLines[:matchIdx]...)
	newLines = append(newLines, withLines...)
	newLines = append(newLines, origLines[matchIdx:]...)

	res := strings.Join(newLines, "\n")
	if isCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
}

func isEllipsisLine(line string) bool {
	t := strings.TrimSpace(line)
	return t == "..." || t == "// ..." || t == "/* ... */" || t == "# ..." || t == "<!-- ... -->" || t == "/*...*/" || t == ".." || t == "//..."
}

func hasEllipsis(s string) bool {
	for _, l := range strings.Split(s, "\n") {
		if isEllipsisLine(l) {
			return true
		}
	}
	return false
}

func replaceWithEllipsis(origStr, search, with string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")
	searchLines := strings.Split(strings.ReplaceAll(search, "\r\n", "\n"), "\n")

	var segments [][]string
	var currSegment []string
	for _, l := range searchLines {
		if isEllipsisLine(l) {
			if len(currSegment) > 0 {
				segments = append(segments, currSegment)
				currSegment = nil
			}
		} else {
			currSegment = append(currSegment, l)
		}
	}
	if len(currSegment) > 0 {
		segments = append(segments, currSegment)
	}

	if len(segments) == 0 {
		return "", fmt.Errorf("search block contains only wildcards")
	}

	var segmentSpans [][2]int
	cursor := 0

	for sIdx, seg := range segments {
		found := -1
		for i := cursor; i <= len(origLines)-len(seg); i++ {
			matched := true
			for j := 0; j < len(seg); j++ {
				if strings.TrimSpace(origLines[i+j]) != strings.TrimSpace(seg[j]) {
					matched = false
					break
				}
			}
			if matched {
				found = i
				break
			}
		}
		if found == -1 {
			return "", fmt.Errorf("wildcard segment #%d (%q) could not be matched after line %d", sIdx+1, strings.TrimSpace(seg[0]), cursor+1)
		}
		segmentSpans = append(segmentSpans, [2]int{found, found + len(seg)})
		cursor = found + len(seg)
	}

	var capturedBlocks []string
	for i := 0; i < len(segmentSpans)-1; i++ {
		start := segmentSpans[i][1]
		end := segmentSpans[i+1][0]
		captured := strings.Join(origLines[start:end], "\n")
		capturedBlocks = append(capturedBlocks, captured)
	}

	withLines := strings.Split(strings.ReplaceAll(with, "\r\n", "\n"), "\n")
	var finalWithLines []string
	captIdx := 0

	for _, l := range withLines {
		if isEllipsisLine(l) {
			if captIdx < len(capturedBlocks) {
				if capturedBlocks[captIdx] != "" {
					finalWithLines = append(finalWithLines, capturedBlocks[captIdx])
				}
				captIdx++
			}
		} else {
			finalWithLines = append(finalWithLines, l)
		}
	}

	totalStart := segmentSpans[0][0]
	totalEnd := segmentSpans[len(segmentSpans)-1][1]

	newLines := make([]string, 0, totalStart+len(finalWithLines)+(len(origLines)-totalEnd))
	newLines = append(newLines, origLines[:totalStart]...)
	newLines = append(newLines, finalWithLines...)
	if totalEnd < len(origLines) {
		newLines = append(newLines, origLines[totalEnd:]...)
	}

	res := strings.Join(newLines, "\n")
	if isCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
}

func findMismatchHint(origStr, search string) string {
	searchLines := strings.Split(strings.ReplaceAll(search, "\r\n", "\n"), "\n")
	if len(searchLines) == 0 {
		return ""
	}
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")

	firstSearch := strings.TrimSpace(searchLines[0])
	if firstSearch == "" {
		return ""
	}

	for i, oLine := range origLines {
		if strings.TrimSpace(oLine) == firstSearch {
			for j := 1; j < len(searchLines) && i+j < len(origLines); j++ {
				s := strings.TrimRight(searchLines[j], " \t\r")
				f := strings.TrimRight(origLines[i+j], " \t\r")
				if s != f {
					return fmt.Sprintf("near target line %d:\n       Search: %q\n       Actual: %q", i+j+1, searchLines[j], origLines[i+j])
				}
			}
		}
	}
	return ""
}
