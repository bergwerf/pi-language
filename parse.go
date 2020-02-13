package main

import (
	"encoding/hex"
	"fmt"
)

// Parse converts a token list into a process.
func Parse(tokens []Token, refCount uint, bound map[string]uint, err *ErrorList) ([]*Proc, []Token) {
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
			parsed, remainder := Parse(tokens, refCount, copyMap(bound), err)
			proc = append(proc, parsed...)
			tokens = remainder
			if len(tokens) == 0 {
				err.Add(fmt.Errorf("%v; missing closing parenthesis", loc))
				break
			}
		}
		return proc, tokens[1:]
	}
	// Find action.
	for _, trans := range coreSyntax {
		m := trans.Pattern.FindStringSubmatch(tokens[0].Content)
		if len(m) > 0 {
			// Resolve or bind names in pattern.
			v := make([]Var, len(m)-1)
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
					v[i] = Var{false, refCount, name}
					bound[name] = refCount
					refCount++
				}
			}

			// Build process and decrement reference count after a PIPopRef.
			proc := trans.Process(loc, v)
			if proc.Action == PIPopRef {
				refCount--
			}

			// Next we expect a ; or .
			tokens = tokens[1:]
			if len(tokens) == 0 {
				err.Add(fmt.Errorf("%v; expected semicolon or period", loc))
			} else if tokens[0].Content == sSemicolon {
				children, remainder := Parse(tokens[1:], refCount, bound, err)
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
	return Parse(tokens[1:], refCount, bound, err)
}

// Check if a name is bound or if it is an interface channel.
func resolveName(name string, bound map[string]uint) (Var, error) {
	// Check if the name is bound.
	if index, in := bound[name]; in {
		return Var{false, index, name}, nil
	}
	// Hexadecimal stdin/stdout
	offset := uint(0)
	m := stdinHexRE.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutIDOffset
		m = stdoutHexRE.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		v, _ := hex.DecodeString(m[1])
		return Var{true, offset + uint(v[0]), name}, nil
	}
	// Alphanumeric stdin/stdout
	offset = 0
	m = stdinAlphaNumRE.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutIDOffset
		m = stdoutAlphaNumRE.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		b := byte(m[1][0])
		return Var{true, offset + uint(b), name}, nil
	}
	// Other special channels
	for k, id := range specialChannels {
		if name == k {
			return Var{true, id, name}, nil
		}
	}
	// Note that Var{true, 0} is always valid.
	return Var{true, 0, name}, fmt.Errorf("unbound variable")
}

// ErrorList keeps track of errors using this error list.
type ErrorList []error

// Add error.
func (l *ErrorList) Add(err error) {
	if err != nil {
		*l = append(*l, err)
	}
}
