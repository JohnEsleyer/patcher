package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// applyReplace executes targeted pattern replacement, symbol replacement, line-range replacement,
// ellipsis wildcard replacement, or 4-tier fuzzy matching.
func applyReplace(origStr string, act Action) (string, error) {
	// 1. Explicit line range replacement: lines="50-120"
	if act.Lines != "" {
		return replaceLines(origStr, act.Lines, act.With)
	}

	// 2. Symbol extraction replacement: symbol="MyFunction"
	if act.Symbol != "" {
		return replaceSymbol(origStr, act.Symbol, act.With)
	}

	// 3. Pattern-bounded replacement: from="start_pattern" to="end_pattern"
	if act.From != "" {
		return replaceFromTo(origStr, act.From, act.To, act.With)
	}

	// 4. Anchor insertion after match: after="pattern"
	if act.After != "" {
		return insertAfter(origStr, act.After, act.With)
	}

	// 5. Anchor insertion before match: before="pattern"
	if act.Before != "" {
		return insertBefore(origStr, act.Before, act.With)
	}

	// 6. Ellipsis / wildcard matching inside <search>: "..."
	if hasEllipsis(act.Search) {
		return replaceWithEllipsis(origStr, act.Search, act.With)
	}

	// 7. Standard 4-tier fuzzy match
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

// performReplace uses a 4-tier fuzzy match: exact, CRLF-normalized, line-trimmed, and indentation-tolerant
func performReplace(origStr, search, with string) (string, bool) {
	// Tier 1: Exact match
	if strings.Contains(origStr, search) {
		return strings.Replace(origStr, search, with, 1), true
	}

	// Tier 2: CRLF / LF line ending normalization
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

	// Tier 3: Trailing whitespace tolerance on lines
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

	// Tier 4: Indentation tolerance (TrimSpace block matching)
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
	isCRLF := strings.Contains(origStr, "\r\n")
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

	if startLine < 1 {
		startLine = 1
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > totalLines || startLine > endLine {
		return "", fmt.Errorf("line range %s is outside file bounds (total lines: %d)", linesSpec, totalLines)
	}

	withLines := strings.Split(strings.ReplaceAll(with, "\r\n", "\n"), "\n")

	newLines := make([]string, 0, (startLine-1)+len(withLines)+(totalLines-endLine))
	newLines = append(newLines, origLines[:startLine-1]...)
	newLines = append(newLines, withLines...)
	if endLine < totalLines {
		newLines = append(newLines, origLines[endLine:]...)
	}

	res := strings.Join(newLines, "\n")
	if isCRLF {
		res = strings.ReplaceAll(res, "\n", "\r\n")
	}
	return res, nil
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

func replaceSymbol(origStr, symbol, with string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")

	prefixes := []string{
		"func ", "fn ", "pub fn ", "type ", "struct ", "interface ",
		"class ", "def ", "trait ", "enum ", "export function ", "export class ",
		"export default function ", "export const ", "const ", "var ", "let ",
	}

	startIdx := -1
	endIdx := -1

	for idx, line := range origLines {
		trimmed := strings.TrimSpace(line)
		isDecl := false
		for _, pfx := range prefixes {
			if strings.HasPrefix(trimmed, pfx) {
				isDecl = true
				break
			}
		}
		if !isDecl {
			continue
		}

		if strings.Contains(trimmed, symbol) {
			startIdx = idx
			endIdx = idx

			// Python-style indentation
			if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "class ") {
				baseIndent := len(line) - len(strings.TrimLeft(line, " \t"))
				for j := idx + 1; j < len(origLines); j++ {
					lTrim := strings.TrimSpace(origLines[j])
					if lTrim == "" {
						continue
					}
					indent := len(origLines[j]) - len(strings.TrimLeft(origLines[j], " \t"))
					if indent > baseIndent {
						endIdx = j
					} else {
						break
					}
				}
				break
			}

			// Brace-style languages
			braceCount := 0
			foundBrace := false
			for j := idx; j < len(origLines); j++ {
				l := origLines[j]
				for _, ch := range l {
					if ch == '{' {
						braceCount++
						foundBrace = true
					} else if ch == '}' {
						braceCount--
					}
				}
				if foundBrace && braceCount <= 0 {
					endIdx = j
					break
				}
				if !foundBrace && strings.HasSuffix(strings.TrimSpace(l), ";") {
					endIdx = j
					break
				}
			}
			break
		}
	}

	if startIdx == -1 {
		return "", fmt.Errorf("symbol %q declaration not found", symbol)
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

	// Split search into non-ellipsis block segments
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

	// Capture wildcard text between matched segments
	var capturedBlocks []string
	for i := 0; i < len(segmentSpans)-1; i++ {
		start := segmentSpans[i][1]
		end := segmentSpans[i+1][0]
		captured := strings.Join(origLines[start:end], "\n")
		capturedBlocks = append(capturedBlocks, captured)
	}

	// If with contains ellipsis, interpolate the captured blocks
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

// findMismatchHint inspects where a search target almost matched to give actionable diagnostics
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