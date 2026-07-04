// Package util provides general utility helper functions used across the application.
package util

import (
	"regexp"
	"strings"
)

// FormatDiff highlights diff files with simple rules and cleans up temporary paths.
func FormatDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")
	for i, line := range lines {
		escaped := strings.ReplaceAll(line, "[", "[[")
		if strings.HasPrefix(line, "--- ") {
			lines[i] = "[yellow]--- local/PKGBUILD[-]"
		} else if strings.HasPrefix(line, "+++ ") {
			lines[i] = "[yellow]+++ remote/PKGBUILD[-]"
		} else if strings.HasPrefix(line, "@@") {
			lines[i] = "[teal]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "-") {
			lines[i] = "[red]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "+") {
			lines[i] = "[green]" + escaped + "[-]"
		} else {
			lines[i] = escaped
		}
	}
	return strings.Join(lines, "\n")
}

// FormatPKGBUILD highlights PKGBUILD script contents using simple rules.
func FormatPKGBUILD(text string) string {
	lines := strings.Split(text, "\n")
	varAssign := regexp.MustCompile(`^(\s*)([a-zA-Z_][a-zA-Z0-9_]*)=(.*)$`)

	tokenRegex := regexp.MustCompile(
		`(# .*)|` + // 1. Comments
			`("[^"\\]*(?:\\.[^"\\]*)*")|` + // 2. Double-quoted strings
			`('[^'\\]*(?:\\.[^'\\]*)*')|` + // 3. Single-quoted strings
			`(\b\d+(?:\.\d+)*\b)|` + // 4. Numbers
			`([&(){}[\]|$<>;=+\-*/:@,.])|` + // 5. Special Characters
			`([a-zA-Z_][a-zA-Z0-9_]*)`, // 6. Words
	)

	for i, line := range lines {
		if matches := varAssign.FindStringSubmatch(line); len(matches) == 4 {
			spaces := matches[1]
			varName := matches[2]
			rest := matches[3]

			lines[i] = spaces + "[teal]" + varName + "[-][fuchsia]=[-]" + tokenizeAndFormat(rest, tokenRegex)
			continue
		}

		lines[i] = tokenizeAndFormat(line, tokenRegex)
	}
	return strings.Join(lines, "\n")
}

// tokenizeAndFormat splits a line into styled segments using the precompiled token regular expression
func tokenizeAndFormat(line string, tokenRegex *regexp.Regexp) string {
	var sb strings.Builder
	n := len(line)

	matches := tokenRegex.FindAllStringSubmatchIndex(line, -1)

	lastIdx := 0
	for _, m := range matches {
		start := m[0]
		end := m[1]

		if start > lastIdx {
			val := line[lastIdx:start]
			val = strings.ReplaceAll(val, "[", "[[")
			sb.WriteString(val)
		}

		matched := false
		for g := 1; g <= 7; g++ {
			gStart := m[2*g]
			gEnd := m[2*g+1]
			if gStart != -1 && gEnd != -1 {
				tokenText := line[gStart:gEnd]
				escapedToken := strings.ReplaceAll(tokenText, "[", "[[")

				switch g {
				case 1: // Comment
					sb.WriteString("[gray]" + escapedToken + "[-]")
				case 2: // Double-quoted string with variable expansion support
					sb.WriteString(formatDoubleQuotedString(tokenText))
				case 3: // Single-quoted String
					sb.WriteString("[yellow]" + escapedToken + "[-]")
				case 4: // Number
					sb.WriteString("[blue]" + escapedToken + "[-]")
				case 5: // Special Character
					sb.WriteString("[fuchsia]" + escapedToken + "[-]")
				case 6: // Word (default/white)
					sb.WriteString(escapedToken)
				}
				matched = true
				break
			}
		}

		if !matched {
			val := line[start:end]
			val = strings.ReplaceAll(val, "[", "[[")
			sb.WriteString(val)
		}

		lastIdx = end
	}

	if lastIdx < n {
		val := line[lastIdx:n]
		val = strings.ReplaceAll(val, "[", "[[")
		sb.WriteString(val)
	}

	return sb.String()
}

// formatDoubleQuotedString highlights double-quoted strings, styling internal variable expansions ($var and ${var}) differently
func formatDoubleQuotedString(strText string) string {
	runes := []rune(strText)
	n := len(runes)
	var sb strings.Builder

	sb.WriteString("[yellow]\"[-]")

	isWordChar := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
	}

	i := 1
	var currentText strings.Builder
	flushText := func() {
		if currentText.Len() > 0 {
			sb.WriteString("[yellow]" + strings.ReplaceAll(currentText.String(), "[", "[[") + "[-]")
			currentText.Reset()
		}
	}

	for i < n-1 {
		r := runes[i]
		if r == '$' {
			flushText()
			sb.WriteString("[fuchsia]$[-]")
			i++
			if i < n-1 && runes[i] == '{' {
				sb.WriteString("[fuchsia]{[-]")
				i++
				var varName strings.Builder
				for i < n-1 && runes[i] != '}' {
					varName.WriteRune(runes[i])
					i++
				}
				sb.WriteString("[teal]" + strings.ReplaceAll(varName.String(), "[", "[[") + "[-]")
				if i < n-1 && runes[i] == '}' {
					sb.WriteString("[fuchsia]}[-]")
					i++
				}
			} else {
				var varName strings.Builder
				for i < n-1 && isWordChar(runes[i]) {
					varName.WriteRune(runes[i])
					i++
				}
				sb.WriteString("[teal]" + strings.ReplaceAll(varName.String(), "[", "[[") + "[-]")
			}
		} else {
			if r == '\\' && i+1 < n-1 {
				currentText.WriteRune(r)
				currentText.WriteRune(runes[i+1])
				i += 2
			} else {
				currentText.WriteRune(r)
				i++
			}
		}
	}
	flushText()

	sb.WriteString("[yellow]\"[-]")
	return sb.String()
}
