package main

import (
	"encoding/hex"
	"fmt"
)

// Parse converts a token list into a process.
func Parse(tokens []Token, refOffset uint, bound map[string]uint, err *ErrorList) ([]*Proc, []Token) {
	if len(tokens) == 0 {
		return nil, nil
	}

	// Expect either a block "(" or an action.
	loc := tokens[0].Location
	if tokens[0].Content == sParOpen {
		// Get processes until first ")".
		proc := make([]*Proc, 0)
		tokens = tokens[1:]
		for tokens[0].Content != sParClose {
			parsed, remainder := Parse(tokens, refOffset, copyMap(bound), err)
			proc = append(proc, parsed...)
			tokens = remainder
			if len(tokens) == 0 {
				err.Add(fmt.Errorf("%v; missing closing parenthesis", loc))
				return proc, nil
			}
		}
		return proc, tokens[1:]
	}
	// Find action.
	for _, trans := range coreSyntax {
		m := trans.Pattern.FindStringSubmatch(tokens[0].Content)
		if len(m) > 0 {
			// Resolve or bind names in pattern.
			v := make([]uint, len(m)-1)
			for i, name := range m[1:] {
				if trans.BindVar>>i == 0 {
					// Resolve.
					vi, vErr := resolveName(name, bound)
					v[i] = vi
					if vErr != nil {
						err.Add(fmt.Errorf("%v; %v; %v", loc, name, vErr))
					}
				} else {
					// Bind.
					bound[name] = refOffset
					v[i] = refOffset
					refOffset++
				}
			}

			// Next we expect a ; or .
			proc := trans.Process(loc, v)
			tokens = tokens[1:]
			if len(tokens) == 0 {
				err.Add(fmt.Errorf("%v; expected semicolon or period", loc))
			} else if tokens[0].Content == sSemicolon {
				children, remainder := Parse(tokens[1:], refOffset, bound, err)
				proc.Children = children
				return []*Proc{proc}, remainder
			} else if tokens[0].Content != sPeriod {
				loc := tokens[0].Location
				err.Add(fmt.Errorf("%v; expected semicolon or period", loc))
			}
			return []*Proc{proc}, tokens[1:]
		}
	}
	// Invalid token.
	err.Add(fmt.Errorf("%v; \"%v\" cannot be parsed", loc, tokens[0].Content))
	return Parse(tokens[1:], refOffset, bound, err)
}

// Check if a name is bound or if it is an IO channel.
func resolveName(name string, bound map[string]uint) (uint, error) {
	// Check if the name is bound.
	if index, isBound := bound[name]; isBound {
		return index, nil
	}
	// Hexadecimal stdin/stdout
	offset := uint(0)
	m := stdinHexRE.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutOffset
		m = stdoutHexRE.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		v, _ := hex.DecodeString(m[1])
		return offset + uint(v[0]), nil
	}
	// Alphanumeric stdin/stdout
	offset = 0
	m = stdinAlphaNumRE.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutOffset
		m = stdoutAlphaNumRE.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		b := byte(m[1][0])
		return offset + uint(b), nil
	}
	// Other IO channels
	for k, index := range miscIOChannels {
		if name == k {
			return index, nil
		}
	}
	// Note that 0 refers to the NIL stdin channel.
	return 0, fmt.Errorf("unbound variable")
}

// ErrorList keeps track of errors using this error list.
type ErrorList []error

// Add error.
func (l *ErrorList) Add(err error) {
	if err != nil {
		*l = append(*l, err)
	}
}
