package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Line comment flag.
const lineComment = '!'

// Directive pattern.
var directiveRe, _ = regexp.Compile("^#(.+):(.*)$")

// Allowed variable name pattern.
var nameRe, _ = regexp.Compile("^[a-zA-Z0-9_]*$")

// Tokens
const (
	Plus = iota
	ReceiveOneSymbol
	ReceiveAllSymbol
	SendSymbol
	Semicolon
	Period
	ParOpen
	ParClose
	Variable
	EOFMark
)

// AST nodes
const (
	ASTCreate = iota
	ASTReceiveOne
	ASTReceiveAll
	ASTSend
)

// A token
type tok struct {
	t       int
	content string
}

// Process is the primary AST node for PI programs.
type Process struct {
	Type     int        // Node type
	L, R     []string   // Variable names
	Children []*Process // Child processes
}

// Parse parses PI program code and returns an AST.
func Parse(source string) ([]*Process, []error) {
	// Tokenize and parse input. To simplify parsing we add parentheses around the
	// entire source (allowing parallel processes in the global scope), and a
	// padding tokens at the end (for an easier out of bounds check).
	open, close, eof := tok{ParOpen, ""}, tok{ParClose, ""}, tok{EOFMark, ""}
	tokens1 := tokenize(source)
	tokens2 := append(append([]tok{open}, tokens1...), close, eof)
	err := errorList([]error{})
	proc, index := parse(tokens2, 0, &err)

	// Check that we finished at the EOF.
	if index+1 != len(tokens2) {
		err = append(err, fmt.Errorf("parser finished before EOF"))
	}

	return proc, err
}

// Parse tokens and return AST. This validates variable names. This parser is a
// bit ad-hoc (but minimal), using a DFA would be nicer.
//
// Slightly more variations than the formal grammar are allowed: receive and
// bind statements without subsequent process, no Semicolon before a ParOpen,
// duplicate Semicolon or Period.
func parse(tokens []tok, index int, err *errorList) ([]*Process, int) {
	// Check bounds. Note that we always end with ParClose and EOF. The ParOpen
	// case already generates an error for this case, we only move the index to
	// the EOF token.
	if index+2 >= len(tokens) {
		return nil, len(tokens) - 1
	}

	switch tokens[index].t {
	// Plus: channel creation.
	case Plus:
		names := splitVariable(tokens[index+1], false, err)
		children, end := parse(tokens, index+2, err)
		return []*Process{&Process{ASTCreate, nil, names, children}}, end

	// Semicolon: return process after semi-colon.
	case Semicolon:
		return parse(tokens, index+1, err)

	// Period: return empty process list.
	case Period:
		return nil, index + 1

	// Variable: expect a bind, receive or send.
	case Variable:
		l := splitVariable(tokens[index], true, err)
		r := splitVariable(tokens[index+2], false, err)
		c, end := parse(tokens, index+3, err)

		switch tokens[index+1].t {
		case ReceiveOneSymbol:
			return []*Process{&Process{ASTReceiveOne, l, r, c}}, end
		case ReceiveAllSymbol:
			return []*Process{&Process{ASTReceiveAll, l, r, c}}, end
		case SendSymbol:
			return []*Process{&Process{ASTSend, l, r, c}}, end
		default:
			err.Add(fmt.Errorf("unexpected token"))
			return nil, index
		}

	// ParOpen: aggregate all processes until the first ParClose.
	case ParOpen:
		index++
		all := make([]*Process, 0)
		for tokens[index].t != ParClose {
			children, newIndex := parse(tokens, index, err)
			all = append(all, children...)

			// If the index did not move forward we got stuck (by a syntax error). We
			// could quit parsing right now, but instead we try if we can pick up a
			// valid trail again by skipping the current token (quicker debugging).
			index = pick(newIndex == index, newIndex+1, newIndex)

			// Check if a closing parenthesis was missing or we ran out of tokens.
			if tokens[index].t == EOFMark {
				err.Add(fmt.Errorf("unexpected EOF"))
				return all, index
			}
		}
		return all, index + 1

	default:
		err.Add(fmt.Errorf("unexpected token"))
		return nil, index
	}
}

// Extract a list of valid names from the given token and return it. To make
// splitting easier the comma is not parsed as a separate token by tokenize.
func splitVariable(variable tok, allowEmpty bool, err *errorList) []string {
	if variable.t != Variable {
		err.Add(fmt.Errorf("expected variable token"))
		return nil
	}
	names := strings.Split(variable.content, ",")
	valid := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !allowEmpty && len(name) == 0 {
			err.Add(fmt.Errorf("illegal empty name"))
		} else if !nameRe.MatchString(name) {
			err.Add(fmt.Errorf("name \"%v\" does not match %v", name, nameRe.String()))
		} else {
			valid = append(valid, name)
		}
	}
	return valid
}

// Extract source tokens and remove whitespace and comments. Illegal variable
// names or other bad syntax are not handled by this function. This step is
// mundane but allows for a cleaner parsing algorithm.
func tokenize(source string) []tok {
	tokens := make([]tok, 0)
	acc := ""
	comment := false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if comment {
			comment = (c != '\n')
			continue
		}
		v := tok{Variable, strings.TrimSpace(acc)}
		switch c {
		case lineComment:
			comment = true
		case '+':
			tokens = append(tokens, tok{Plus, ""})
		case ';':
			tokens = append(tokens, v, tok{Semicolon, ""})
			acc = ""
		case '.':
			tokens = append(tokens, v, tok{Period, ""})
			acc = ""
		case '(':
			tokens = append(tokens, tok{ParOpen, ""})
		case ')':
			tokens = append(tokens, tok{ParClose, ""})
		default:
			if len(source) <= i+2 {
				acc += string(c)
				continue
			}
			switch source[i : i+2] {
			case "<-":
				tokens = append(tokens, v, tok{ReceiveOneSymbol, ""})
				acc = ""
				i++
			case "<<":
				tokens = append(tokens, v, tok{ReceiveAllSymbol, ""})
				acc = ""
				i++
			case "->":
				tokens = append(tokens, v, tok{SendSymbol, ""})
				acc = ""
				i++
			default:
				acc += string(c)
			}
		}
	}
	return tokens
}

// ExtractDirectives removes directives appearing at the beginning of the given
// source and returns them as a map with the remaining code. Directives can only
// occur before any PI script and do not depend on each other. Comments and
// empty lines are allowed between directives.
func ExtractDirectives(source string) ([]string, []string, string) {
	lines := strings.Split(source, "\n")
	attach, global := make([]string, 0), make([]string, 0)
	for i, line := range lines {
		line = strings.TrimSpace(line)
		m := directiveRe.FindStringSubmatch(line)
		if len(m) > 0 {
			k := m[1]
			v := strings.TrimSpace(strings.Split(m[2], string(lineComment))[0])
			switch k {
			case "attach":
				attach = append(attach, v)
			case "global":
				global = append(global, v)
			}
		} else if len(line) == 0 || line[0] == lineComment {
			// Skip empty lines or comments.
			continue
		} else {
			// End of directives; return result.
			return attach, global, strings.Join(lines[i:], "\n")
		}
	}
	return attach, global, ""
}
