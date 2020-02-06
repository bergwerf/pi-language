package main

import (
	"fmt"
	"regexp"
	"strings"
)

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
	proc, index, err := parse(tokens2, 0)

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
func parse(tokens []tok, index int) ([]*Process, int, []error) {
	// Check bounds. Note that we always end with ParClose and EOF. The ParOpen
	// case already generates an error for this case, we only move the index to
	// the EOF token.
	if index+2 >= len(tokens) {
		return nil, len(tokens) - 1, nil
	}

	switch tokens[index].t {
	// Plus: channel creation.
	case Plus:
		names, err1 := splitVariable(tokens[index+1], false)
		children, end, err2 := parse(tokens, index+2)
		err := append(err1, err2...)
		return []*Process{&Process{ASTCreate, nil, names, children}}, end, err

	// Semicolon: return process after semi-colon.
	case Semicolon:
		return parse(tokens, index+1)

	// Period: return empty process list.
	case Period:
		return nil, index + 1, nil

	// Variable: expect a bind, receive or send.
	case Variable:
		l, err1 := splitVariable(tokens[index], true)
		r, err2 := splitVariable(tokens[index+2], false)
		c, end, err3 := parse(tokens, index+3)
		err := append(append(err1, err2...), err3...)

		switch tokens[index+1].t {
		case ReceiveOneSymbol:
			return []*Process{&Process{ASTReceiveOne, l, r, c}}, end, err
		case ReceiveAllSymbol:
			return []*Process{&Process{ASTReceiveAll, l, r, c}}, end, err
		case SendSymbol:
			return []*Process{&Process{ASTSend, l, r, c}}, end, err
		default:
			return nil, index, []error{fmt.Errorf("unexpected token")}
		}

	// ParOpen: aggregate all processes until the first ParClose.
	case ParOpen:
		index++
		all := make([]*Process, 0)
		err := make([]error, 0)
		for tokens[index].t != ParClose {
			children, index1, err1 := parse(tokens, index)
			index = index1 // Golang would otherwise shadow index.
			all = append(all, children...)
			err = append(err, err1...)
			// Check if a closing parenthesis was missing or we ran out of tokens.
			if tokens[index].t == EOFMark {
				return all, index, append(err, fmt.Errorf("unexpected EOF"))
			}
		}
		return all, index + 1, err

	default:
		return nil, index, []error{fmt.Errorf("unexpected token")}
	}
}

// Extract a list of valid names from the given token and return it. To make
// splitting easier the comma is not parsed as a separate token by tokenize.
func splitVariable(variable tok, allowEmpty bool) ([]string, []error) {
	if variable.t != Variable {
		return nil, []error{fmt.Errorf("expected variable token")}
	}
	names := strings.Split(variable.content, ",")
	valid := make([]string, 0, len(names))
	errors := make([]error, 0)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !allowEmpty && len(name) == 0 {
			err := fmt.Errorf("illegal empty name")
			errors = append(errors, err)
		} else if !nameRe.MatchString(name) {
			err := fmt.Errorf("name \"%v\" does not match %v", name, nameRe.String())
			errors = append(errors, err)
		} else {
			valid = append(valid, name)
		}
	}
	return valid, errors
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
			switch source[i : i+2] {
			case "--":
				comment = true
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
