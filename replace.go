package main

import (
	"fmt"
	"strings"
)

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
