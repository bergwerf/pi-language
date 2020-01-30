package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Allowed variable name pattern.
var nameRe, _ = regexp.Compile("[a-zA-Z0-9]+")

// Tokens
const (
	Plus = iota
	LeftArrow
	RightArrow
	SemiColon
	Period
	Star
	ParOpen
	ParClose
	Variable
	EOFMark
)

// AST nodes
const (
	CreateChannel = iota
	ReceiveChannel
	SendChannel
	ReplicateProcess
)

// A token
type tok struct {
	t    int
	name string
}

// Process is the primary AST node for PI programs. I considered using AST
// pointers to be pre-mature optimization.
type Process struct {
	t    int       // Node type
	x, y string    // Variables
	next []Process // Next processes
}

// Parse parses PI program code and returns an AST.
func Parse(source string) ([]Process, []error) {
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
func parse(tokens []tok, index int) ([]Process, int, []error) {
	// Check bounds. Note that we always end with ParClose and EOF. The ParOpen
	// case already generates an error for this case, we only move the index to
	// the EOF token.
	if index+2 >= len(tokens) {
		return []Process{}, len(tokens) - 1, []error{}
	}

	switch tokens[index].t {
	// Plus: channel creation.
	case Plus:
		name, err1 := checkVariable(tokens[index+1])
		next, end, err2 := parse(tokens, index+2)
		err := mergeErr(err1, err2)
		return []Process{Process{CreateChannel, name, "", next}}, end, err

	// Semicolon: return process after semi-colon.
	case SemiColon:
		return parse(tokens, index+1)

	// Period: return empty process list.
	case Period:
		return []Process{}, index + 1, []error{}

	// Variable: expect a receive or send.
	case Variable:
		x, err1 := checkVariable(tokens[index])
		y, err2 := checkVariable(tokens[index+2])
		next, end, err3 := parse(tokens, index+3)
		err := mergeErr(err1, mergeErr(err2, err3))

		switch tokens[index+1].t {
		case LeftArrow:
			return []Process{Process{ReceiveChannel, x, y, next}}, end, err
		case RightArrow:
			return []Process{Process{SendChannel, x, y, next}}, end, err
		default:
			return []Process{}, index, []error{fmt.Errorf("unexpected token")}
		}

	// Star: mark next process as replicated.
	case Star:
		next, end, err := parse(tokens, index+1)
		return []Process{Process{ReplicateProcess, "", "", next}}, end, err

	// ParOpen: aggregate all processes until the first ParClose.
	case ParOpen:
		index++
		all := make([]Process, 0)
		err := make([]error, 0)
		for tokens[index].t != ParClose {
			next, index1, err1 := parse(tokens, index)
			index = index1 // Prevent shadowing of index.
			all = append(all, next...)
			err = append(err, err1...)
			// Check if a closing parenthesis was missing or we ran out of tokens.
			if tokens[index].t == EOFMark {
				return all, index, append(err, fmt.Errorf("unexpected EOF"))
			}
		}
		return all, index + 1, err

	default:
		return []Process{}, index, []error{fmt.Errorf("unexpected token")}
	}
}

// Helper to combine errors.
func mergeErr(hd error, tl []error) []error {
	if hd != nil {
		return append([]error{hd}, tl...)
	}
	return tl
}

// Check variable token
func checkVariable(variable tok) (string, error) {
	if variable.t != Variable {
		return "", fmt.Errorf("expected variable token")
	}
	name := variable.name
	if !nameRe.MatchString(name) {
		err := fmt.Errorf("name \"%v\" does not match %v", name, nameRe.String())
		return name, err
	}
	return name, nil
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
			tokens = append(tokens, v, tok{SemiColon, ""})
			acc = ""
		case '.':
			tokens = append(tokens, v, tok{Period, ""})
			acc = ""
		case '*':
			tokens = append(tokens, tok{Star, ""})
		case '(':
			tokens = append(tokens, tok{ParOpen, ""})
		case ')':
			tokens = append(tokens, tok{ParClose, ""})
		default:
			switch source[i : i+2] {
			case "--":
				comment = true
			case "<-":
				tokens = append(tokens, v, tok{LeftArrow, ""})
				acc = ""
				i++
			case "->":
				tokens = append(tokens, v, tok{RightArrow, ""})
				acc = ""
				i++
			default:
				acc += string(c)
			}
		}
	}
	return tokens
}
