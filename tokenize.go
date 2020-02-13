package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Token is an intermediate normal form piece.
type Token struct {
	Location Loc    // location in original source
	Content  string // action or control character (;.)
}

// Regular expressions for tokenization
var (
	whitespaceRE, _    = regexp.Compile("^\\s*")
	controlPrefixRE, _ = regexp.Compile(fmt.Sprintf("^%v", control))
	controlNextRE, _   = regexp.Compile(fmt.Sprintf("^(.*?)(?:%v|$)", control))
)

// Tokenize a PI program. The result is normalized.
func Tokenize(source string, start Loc, relativeLoc bool) []Token {
	tokens := make([]Token, 0)

	// Read line by line for easier location tracking.
	for ln, line := range strings.Split(source, "\n") {
		lineLength := len(line)
		for len(line) > 0 {
			m1 := whitespaceRE.FindString(line) // Skip whitespace.
			line = line[len(m1):]

			// Find token start location.
			loc := start
			if relativeLoc {
				loc.Ln += ln
				loc.Col = lineLength - len(line) + 1
			}

			// Check if this is a control token.
			m2 := controlPrefixRE.FindString(line)
			if len(m2) > 0 {
				tokens = append(tokens, Token{loc, m2})
				line = line[len(m2):]
				continue
			}

			// Check for normalization rule.
			for _, rw := range extendedSyntax {
				m3 := rw.Pattern.FindStringSubmatch(line)
				if len(m3) > 0 {
					parts := make([]interface{}, len(m3))
					for i, str := range m3 {
						parts[i] = str
					}

					replace := fmt.Sprintf(rw.Replace, parts[1:]...)
					result := Tokenize(replace, loc, false)
					tokens = append(tokens, result...)
					line = line[len(m3[0]):]
					continue
				}
			}

			// Skip until next control character.
			m4 := controlNextRE.FindStringSubmatch(line)
			assert(len(m4) > 0)
			if len(m4[1]) > 0 {
				tokens = append(tokens, Token{loc, m4[1]})
				line = line[len(m4[1]):]
			}
		}
	}

	return tokens
}

// ExtractDirectives removes directives appearing at the beginning of the given
// source. Directives can only occur before the PI script and do not depend on
// each other. Comments and empty lines between directives are allowed.
func ExtractDirectives(source string) ([]string, []string, int, string) {
	lines := strings.Split(source, "\n")
	attach, global := make([]string, 0), make([]string, 0)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		m := directiveRE.FindStringSubmatch(line)
		if len(m) > 0 {
			k, v := m[1], strings.TrimSpace(m[2])
			switch k {
			case "attach":
				attach = append(attach, v)
			case "global":
				global = append(global, v)
			}
		} else if len(line) == 0 || line[0:1] == sComment {
			// Skip empty lines or comments.
			continue
		} else {
			// End of directives; return result.
			return attach, global, i, strings.Join(lines[i:], "\n")
		}
	}
	return attach, global, len(lines), ""
}
