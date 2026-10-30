package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"
)

// cleanReceiverName normalizes receiver strings like "(s *Server)", "*Server", "s Server" to "Server".
func cleanReceiverName(r string) string {
	r = strings.TrimSpace(r)
	r = strings.TrimPrefix(r, "(")
	r = strings.TrimSuffix(r, ")")
	r = strings.TrimSpace(r)
	parts := strings.Fields(r)
	if len(parts) > 1 {
		r = parts[len(parts)-1]
	}
	r = strings.TrimPrefix(r, "*")
	return strings.TrimSpace(r)
}

// parseSymbolQuery extracts receiver and member names from a symbol query (e.g. "Server.Start" or "(s *Server).Start").
func parseSymbolQuery(symbol string) (string, string) {
	symbol = strings.TrimSpace(symbol)
	if strings.HasPrefix(symbol, "(") {
		closeIdx := strings.Index(symbol, ")")
		if closeIdx != -1 {
			recv := symbol[1:closeIdx]
			rest := strings.TrimPrefix(symbol[closeIdx+1:], ".")
			return cleanReceiverName(recv), strings.TrimSpace(rest)
		}
	}
	if dotIdx := strings.LastIndex(symbol, "."); dotIdx != -1 {
		recv := symbol[:dotIdx]
		name := symbol[dotIdx+1:]
		return cleanReceiverName(recv), strings.TrimSpace(name)
	}
	return "", symbol
}

// replaceLineSpan replaces 1-based lines startLine through endLine (inclusive) with withStr.
func replaceLineSpan(origStr string, startLine, endLine int, withStr string) (string, error) {
	isCRLF := strings.Contains(origStr, "\r\n")
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")
	totalLines := len(origLines)

	if startLine < 1 {
		startLine = 1
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > totalLines || startLine > endLine {
		return "", fmt.Errorf("line range %d-%d is outside file bounds (total lines: %d)", startLine, endLine, totalLines)
	}

	withLines := strings.Split(strings.ReplaceAll(withStr, "\r\n", "\n"), "\n")
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

type goMatch struct {
	name      string
	receiver  string
	startLine int
	endLine   int
	kind      string
}

// findGoSymbol parses Go source with the standard go/parser and locates the exact start and end lines.
func findGoSymbol(origStr, symbol, withStr string) (int, int, error) {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, "", origStr, parser.ParseComments)
	if err != nil {
		return 0, 0, err
	}

	recvFilter, nameFilter := parseSymbolQuery(symbol)
	withTrimmed := strings.TrimSpace(withStr)
	hasDocInWith := strings.HasPrefix(withTrimmed, "//") || strings.HasPrefix(withTrimmed, "/*")

	var matches []goMatch

	for _, decl := range fileNode.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fnName := d.Name.Name
			var recvStr string
			if d.Recv != nil && len(d.Recv.List) > 0 {
				var buf strings.Builder
				_ = printer.Fprint(&buf, fset, d.Recv.List[0].Type)
				recvStr = cleanReceiverName(buf.String())
			}

			matched := false
			if recvFilter != "" {
				if fnName == nameFilter && recvStr == recvFilter {
					matched = true
				}
			} else {
				if fnName == nameFilter {
					matched = true
				}
			}

			if matched {
				startPos := d.Pos()
				if d.Doc != nil {
					if hasDocInWith {
						startPos = d.Doc.Pos()
					} else {
						startPos = d.Type.Func
					}
				}
				endPos := d.End()
				kind := "function"
				if recvStr != "" {
					kind = "method"
				}
				matches = append(matches, goMatch{
					name:      fnName,
					receiver:  recvStr,
					startLine: fset.Position(startPos).Line,
					endLine:   fset.Position(endPos).Line,
					kind:      kind,
				})
			}

		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if recvFilter == "" && ts.Name.Name == nameFilter {
					var startPos token.Pos
					var endPos token.Pos
					if len(d.Specs) == 1 {
						startPos = d.Pos()
						if d.Doc != nil {
							if hasDocInWith {
								startPos = d.Doc.Pos()
							} else {
								startPos = d.TokPos
							}
						}
						endPos = d.End()
					} else {
						startPos = ts.Pos()
						if ts.Doc != nil {
							if hasDocInWith {
								startPos = ts.Doc.Pos()
							} else {
								startPos = ts.Name.Pos()
							}
						}
						endPos = ts.End()
					}

					kind := "type"
					if _, isStruct := ts.Type.(*ast.StructType); isStruct {
						kind = "struct"
					} else if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
						kind = "interface"
					}

					matches = append(matches, goMatch{
						name:      ts.Name.Name,
						startLine: fset.Position(startPos).Line,
						endLine:   fset.Position(endPos).Line,
						kind:      kind,
					})
				}
			}
		}
	}

	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("symbol %q not found in Go AST", symbol)
	}

	if len(matches) > 1 {
		var descs []string
		for _, m := range matches {
			if m.receiver != "" {
				descs = append(descs, fmt.Sprintf("(%s).%s on line %d", m.receiver, m.name, m.startLine))
			} else {
				descs = append(descs, fmt.Sprintf("%s (%s) on line %d", m.name, m.kind, m.startLine))
			}
		}
		return 0, 0, fmt.Errorf("multiple declarations found for symbol %q: %s; please disambiguate (e.g. symbol=\"Receiver.Method\")", symbol, strings.Join(descs, ", "))
	}

	return matches[0].startLine, matches[0].endLine, nil
}

type tsTokenType int

const (
	tokWord tsTokenType = iota
	tokLBrace
	tokRBrace
	tokLParen
	tokRParen
	tokSemicolon
	tokAssign
	tokArrow
)

type tsToken struct {
	typ     tsTokenType
	val     string
	line    int
	docLine int
}

type tsSymbolMatch struct {
	name      string
	parent    string
	kind      string
	startLine int
	endLine   int
}

// findTypeScriptSymbol tokenizes and parses TypeScript/JavaScript source to find exact symbol line bounds.
func findTypeScriptSymbol(origStr, symbol, withStr string) (int, int, error) {
	tokens := tokenizeTypeScript(origStr)
	recvFilter, nameFilter := parseSymbolQuery(symbol)
	withTrimmed := strings.TrimSpace(withStr)
	hasDocInWith := strings.HasPrefix(withTrimmed, "//") || strings.HasPrefix(withTrimmed, "/*")

	var matches []tsSymbolMatch
	n := len(tokens)

	var currentClass string
	classBraceDepth := 0
	braceDepth := 0

	for i := 0; i < n; i++ {
		tok := tokens[i]

		if tok.typ == tokLBrace {
			braceDepth++
			continue
		}
		if tok.typ == tokRBrace {
			if currentClass != "" && braceDepth == classBraceDepth {
				currentClass = ""
				classBraceDepth = 0
			}
			braceDepth--
			continue
		}

		if tok.typ != tokWord {
			continue
		}

		// 1. Class Declaration
		if tok.val == "class" && i+1 < n && tokens[i+1].typ == tokWord {
			className := tokens[i+1].val
			startLine := tok.line
			if hasDocInWith && tok.docLine > 0 {
				startLine = tok.docLine
			}
			// Search backward on same line for 'export', 'abstract', etc.
			for back := i - 1; back >= 0 && tokens[back].line == tok.line; back-- {
				if tokens[back].val == "export" || tokens[back].val == "abstract" || tokens[back].val == "default" {
					startLine = tokens[back].line
				}
			}

			// Find matching brace for class body
			endLine := tok.line
			for j := i + 2; j < n; j++ {
				if tokens[j].typ == tokLBrace {
					currentClass = className
					classBraceDepth = braceDepth + 1
					endLine = findMatchingBraceTokenLine(tokens, j)
					break
				}
			}

			if (recvFilter == "" || recvFilter == className) && className == nameFilter {
				matches = append(matches, tsSymbolMatch{
					name:      className,
					kind:      "class",
					startLine: startLine,
					endLine:   endLine,
				})
			}
			continue
		}

		// 2. Interface Declaration
		if tok.val == "interface" && i+1 < n && tokens[i+1].typ == tokWord {
			ifaceName := tokens[i+1].val
			startLine := tok.line
			if hasDocInWith && tok.docLine > 0 {
				startLine = tok.docLine
			}
			for back := i - 1; back >= 0 && tokens[back].line == tok.line; back-- {
				if tokens[back].val == "export" {
					startLine = tokens[back].line
				}
			}
			endLine := tok.line
			for j := i + 2; j < n; j++ {
				if tokens[j].typ == tokLBrace {
					endLine = findMatchingBraceTokenLine(tokens, j)
					break
				}
			}
			if (recvFilter == "" || recvFilter == ifaceName) && ifaceName == nameFilter {
				matches = append(matches, tsSymbolMatch{
					name:      ifaceName,
					kind:      "interface",
					startLine: startLine,
					endLine:   endLine,
				})
			}
			continue
		}

		// 3. Type Declaration
		if tok.val == "type" && i+1 < n && tokens[i+1].typ == tokWord {
			typeName := tokens[i+1].val
			startLine := tok.line
			if hasDocInWith && tok.docLine > 0 {
				startLine = tok.docLine
			}
			for back := i - 1; back >= 0 && tokens[back].line == tok.line; back-- {
				if tokens[back].val == "export" {
					startLine = tokens[back].line
				}
			}
			endLine := tok.line
			for j := i + 2; j < n; j++ {
				if tokens[j].typ == tokLBrace {
					endLine = findMatchingBraceTokenLine(tokens, j)
					// check for trailing semicolon
					if j < n-1 && tokens[j+1].typ == tokSemicolon {
						endLine = tokens[j+1].line
					}
					break
				}
				if tokens[j].typ == tokSemicolon {
					endLine = tokens[j].line
					break
				}
			}
			if (recvFilter == "" || recvFilter == typeName) && typeName == nameFilter {
				matches = append(matches, tsSymbolMatch{
					name:      typeName,
					kind:      "type",
					startLine: startLine,
					endLine:   endLine,
				})
			}
			continue
		}

		// 4. Function Declaration
		if tok.val == "function" {
			fnIdx := i + 1
			if fnIdx < n && tokens[fnIdx].val == "*" { // generator
				fnIdx++
			}
			if fnIdx < n && tokens[fnIdx].typ == tokWord {
				fnName := tokens[fnIdx].val
				startLine := tok.line
				if hasDocInWith && tok.docLine > 0 {
					startLine = tok.docLine
				}
				for back := i - 1; back >= 0 && tokens[back].line == tok.line; back-- {
					if tokens[back].val == "export" || tokens[back].val == "default" || tokens[back].val == "async" {
						startLine = tokens[back].line
					}
				}
				endLine := tok.line
				for j := fnIdx + 1; j < n; j++ {
					if tokens[j].typ == tokLBrace {
						endLine = findMatchingBraceTokenLine(tokens, j)
						break
					}
				}
				if (recvFilter == "" || recvFilter == fnName) && fnName == nameFilter {
					matches = append(matches, tsSymbolMatch{
						name:      fnName,
						kind:      "function",
						startLine: startLine,
						endLine:   endLine,
					})
				}
			}
			continue
		}

		// 5. Arrow Function / Constant Function
		if (tok.val == "const" || tok.val == "let" || tok.val == "var") && i+1 < n && tokens[i+1].typ == tokWord {
			varName := tokens[i+1].val
			if i+2 < n && tokens[i+2].typ == tokAssign {
				isFunc := false
				for j := i + 3; j < n && tokens[j].line <= tokens[i].line+2; j++ {
					if tokens[j].typ == tokArrow || tokens[j].val == "function" {
						isFunc = true
						break
					}
					if tokens[j].typ == tokSemicolon {
						break
					}
				}
				if isFunc {
					startLine := tok.line
					if hasDocInWith && tok.docLine > 0 {
						startLine = tok.docLine
					}
					for back := i - 1; back >= 0 && tokens[back].line == tok.line; back-- {
						if tokens[back].val == "export" {
							startLine = tokens[back].line
						}
					}
					endLine := tok.line
					for j := i + 3; j < n; j++ {
						if tokens[j].typ == tokLBrace {
							endLine = findMatchingBraceTokenLine(tokens, j)
							if j < n-1 && tokens[j+1].typ == tokSemicolon {
								endLine = tokens[j+1].line
							}
							break
						}
						if tokens[j].typ == tokSemicolon {
							endLine = tokens[j].line
							break
						}
					}
					if (recvFilter == "" || recvFilter == varName) && varName == nameFilter {
						matches = append(matches, tsSymbolMatch{
							name:      varName,
							kind:      "function",
							startLine: startLine,
							endLine:   endLine,
						})
					}
				}
			}
			continue
		}

		// 6. Class Method
		if currentClass != "" && braceDepth == classBraceDepth {
			methodModifiers := map[string]bool{
				"public": true, "private": true, "protected": true, "static": true, "async": true, "get": true, "set": true,
			}
			targetTokIdx := i
			for targetTokIdx < n && methodModifiers[tokens[targetTokIdx].val] {
				targetTokIdx++
			}
			if targetTokIdx < n && tokens[targetTokIdx].typ == tokWord && targetTokIdx+1 < n && tokens[targetTokIdx+1].typ == tokLParen {
				methodName := tokens[targetTokIdx].val
				startLine := tokens[i].line
				if hasDocInWith && tokens[i].docLine > 0 {
					startLine = tokens[i].docLine
				}
				endLine := tokens[i].line
				for j := targetTokIdx + 2; j < n; j++ {
					if tokens[j].typ == tokLBrace {
						endLine = findMatchingBraceTokenLine(tokens, j)
						break
					}
				}

				matched := false
				if recvFilter != "" {
					if methodName == nameFilter && currentClass == recvFilter {
						matched = true
					}
				} else {
					if methodName == nameFilter {
						matched = true
					}
				}

				if matched {
					matches = append(matches, tsSymbolMatch{
						name:      methodName,
						parent:    currentClass,
						kind:      "method",
						startLine: startLine,
						endLine:   endLine,
					})
				}
				// Skip past the method name so it isn't parsed again as a
				// separate declaration when it was preceded by modifiers.
				i = targetTokIdx
			}
		}
	}

	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("symbol %q not found in TypeScript AST", symbol)
	}

	if len(matches) > 1 {
		var descs []string
		for _, m := range matches {
			if m.parent != "" {
				descs = append(descs, fmt.Sprintf("%s.%s on line %d", m.parent, m.name, m.startLine))
			} else {
				descs = append(descs, fmt.Sprintf("%s (%s) on line %d", m.name, m.kind, m.startLine))
			}
		}
		return 0, 0, fmt.Errorf("multiple declarations found for symbol %q: %s; please disambiguate (e.g. symbol=\"Class.Method\")", symbol, strings.Join(descs, ", "))
	}

	return matches[0].startLine, matches[0].endLine, nil
}

func findMatchingBraceTokenLine(tokens []tsToken, openIdx int) int {
	depth := 0
	for j := openIdx; j < len(tokens); j++ {
		if tokens[j].typ == tokLBrace {
			depth++
		} else if tokens[j].typ == tokRBrace {
			depth--
			if depth <= 0 {
				return tokens[j].line
			}
		}
	}
	return tokens[len(tokens)-1].line
}

func tokenizeTypeScript(src string) []tsToken {
	var tokens []tsToken
	n := len(src)
	i := 0
	line := 1

	lastDocStartLine := 0
	lastDocEndLine := 0

	for i < n {
		ch := src[i]

		if ch == '\n' {
			line++
			i++
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\r' {
			i++
			continue
		}

		// Line comments
		if i+1 < n && ch == '/' && src[i+1] == '/' {
			i += 2
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}

		// Block comments
		if i+1 < n && ch == '/' && src[i+1] == '*' {
			docStart := line
			isDoc := i+2 < n && src[i+2] == '*'
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			if i+1 < n {
				i += 2
			}
			if isDoc {
				lastDocStartLine = docStart
				lastDocEndLine = line
			}
			continue
		}

		// Strings
		if ch == '\'' || ch == '"' {
			quote := ch
			i++
			for i < n && src[i] != quote {
				if src[i] == '\\' && i+1 < n {
					i++
				}
				if src[i] == '\n' {
					line++
				}
				i++
			}
			if i < n {
				i++
			}
			continue
		}

		// Template literals
		if ch == '`' {
			i++
			for i < n && src[i] != '`' {
				if src[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if src[i] == '\n' {
					line++
					i++
					continue
				}
				if i+1 < n && src[i] == '$' && src[i+1] == '{' {
					i += 2
					nested := 1
					for i < n && nested > 0 {
						if src[i] == '\n' {
							line++
						}
						if src[i] == '{' {
							nested++
						}
						if src[i] == '}' {
							nested--
						}
						if src[i] == '\'' || src[i] == '"' {
							q := src[i]
							i++
							for i < n && src[i] != q {
								if src[i] == '\\' && i+1 < n {
									i++
								}
								if src[i] == '\n' {
									line++
								}
								i++
							}
						}
						i++
					}
					continue
				}
				i++
			}
			if i < n {
				i++
			}
			continue
		}

		// Punctuation
		switch ch {
		case '{':
			tokens = append(tokens, tsToken{typ: tokLBrace, val: "{", line: line})
			i++
			continue
		case '}':
			tokens = append(tokens, tsToken{typ: tokRBrace, val: "}", line: line})
			i++
			continue
		case '(':
			tokens = append(tokens, tsToken{typ: tokLParen, val: "(", line: line})
			i++
			continue
		case ')':
			tokens = append(tokens, tsToken{typ: tokRParen, val: ")", line: line})
			i++
			continue
		case ';':
			tokens = append(tokens, tsToken{typ: tokSemicolon, val: ";", line: line})
			i++
			continue
		case '=':
			if i+1 < n && src[i+1] == '>' {
				tokens = append(tokens, tsToken{typ: tokArrow, val: "=>", line: line})
				i += 2
			} else {
				tokens = append(tokens, tsToken{typ: tokAssign, val: "=", line: line})
				i++
			}
			continue
		}

		// Identifiers / Words
		if isIdentStart(ch) {
			start := i
			for i < n && isIdentChar(src[i]) {
				i++
			}
			word := src[start:i]
			docLine := 0
			if lastDocEndLine == line || lastDocEndLine == line-1 {
				docLine = lastDocStartLine
				lastDocStartLine = 0
			}
			tokens = append(tokens, tsToken{typ: tokWord, val: word, line: line, docLine: docLine})
			continue
		}

		i++
	}
	return tokens
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

// findGenericSymbol provides fallback regex and indentation/brace search for non-Go/TS files.
func findGenericSymbol(origStr, symbol string) (int, int, error) {
	origLines := strings.Split(strings.ReplaceAll(origStr, "\r\n", "\n"), "\n")
	_, nameFilter := parseSymbolQuery(symbol)

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

		if strings.Contains(trimmed, nameFilter) {
			startIdx = idx
			endIdx = idx

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
		return 0, 0, fmt.Errorf("symbol %q declaration not found", symbol)
	}

	return startIdx + 1, endIdx + 1, nil
}

// replaceSymbol determines the exact line range for the symbol and applies replacement.
func replaceSymbol(origStr, path, symbol, with string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var startLine, endLine int
	var err error

	switch ext {
	case ".go":
		startLine, endLine, err = findGoSymbol(origStr, symbol, with)
		if err != nil && !strings.Contains(err.Error(), "multiple declarations") {
			if s, e, gErr := findGenericSymbol(origStr, symbol); gErr == nil {
				startLine, endLine, err = s, e, nil
			}
		}
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		startLine, endLine, err = findTypeScriptSymbol(origStr, symbol, with)
		if err != nil && !strings.Contains(err.Error(), "multiple declarations") {
			if s, e, gErr := findGenericSymbol(origStr, symbol); gErr == nil {
				startLine, endLine, err = s, e, nil
			}
		}
	default:
		startLine, endLine, err = findGenericSymbol(origStr, symbol)
	}

	if err != nil {
		return "", err
	}

	return replaceLineSpan(origStr, startLine, endLine, with)
}
