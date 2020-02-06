package main

import (
	"encoding/hex"
	"fmt"
	"regexp"
)

// Interface channels
var (
	// 0..255
	stdinHexRe, _ = regexp.Compile("^stdin_([0-9A-F]{2})$")
	stdinANRe, _  = regexp.Compile("^stdin__([a-zA-Z0-9A-F])$")

	// 256..511
	stdoutIDOffset = uint(256)
	stdoutHexRe, _ = regexp.Compile("^stdout_([0-9A-F]{2})$")
	stdoutANRe, _  = regexp.Compile("^stdout__([a-zA-Z0-9A-F])$")

	// Other channels
	specialChannels = map[string]uint{
		"stdin_read": uint(512),
		"stdin_EOF":  uint(513),
		"debug":      uint(514),
	}

	reservedLength = uint(515)
)

// Proc types
//
// These are named a bit different from Process types to better describe what
// the process does in the simulation rather than what action it describes.
const (
	PINewRef = iota
	PIPopRef
	PISubsOne
	PISubsAll
	PISend
)

// Var represents a variable.
type Var struct {
	Raw   bool   // Raw variable ID (for interface channels)
	Value uint   // Variable index or ID
	Name  string // Name in source code (for debugging)
}

// ID returns the ID this variable is bound to on the given node.
func (v Var) ID(node Node) uint {
	if v.Raw {
		return v.Value
	}
	return node.Refs[v.Value]
}

// Proc represents a node in the processed program AST. Proc is similar to
// Process but contains additional information.
type Proc struct {
	Type     int
	Channel  Var     // Variable for allocated and receive/send channel
	Message  Var     // Variable for receive/send message
	Children []*Proc // Child processes (parallel)
}

// ProcessProgram converts a parsed AST into a program in the core format (this
// includes some desugaring).
func ProcessProgram(program []*Process, seq uint, bound map[string]uint, err *errorList) []*Proc {
	proc := make([]*Proc, 0, len(program))
	for _, p := range program {
		// Unroll argument lists.
		if len(p.L) > 1 || len(p.R) > 1 {
			l1, l2, r1, r2 := p.L, p.L, p.R, p.R
			if len(p.L) > 1 {
				l1, l2 = p.L[0:1], p.L[1:]
			} else {
				r1, r2 = p.R[0:1], p.R[1:]
			}

			pIn := &Process{p.Type, l1, r1, []*Process{&Process{p.Type, l2, r2, p.Children}}}
			pOut := ProcessProgram([]*Process{pIn}, seq, bound, err)

			assert(len(pOut) == 1)
			proc = append(proc, pOut[0])
			continue
		}

		// Require a right argument for all process types.
		if len(p.R) != 1 {
			continue
		}

		switch p.Type {
		case ASTCreate:
			created := Var{false, seq, p.R[0]}
			children := ProcessProgram(p.Children, seq+1, bindName(bound, created), err)
			proc = append(proc, &Proc{PINewRef, created, Var{}, children})

		case ASTReceiveOne:
			fallthrough
		case ASTReceiveAll:
			subscribeType := pick(p.Type == ASTReceiveOne, PISubsOne, PISubsAll)
			if len(p.L) == 1 {
				channel := resolveName(p.R[0], bound, err)
				message := Var{false, seq, p.L[0]}

				if len(p.L[0]) == 0 {
					// Pop message right after receiving it.
					children := ProcessProgram(p.Children, seq, bound, err)
					proc = append(proc, &Proc{subscribeType, channel, message, []*Proc{
						&Proc{PIPopRef, message, Var{}, children},
					}})
				} else {
					// Bind message to the next reference index.
					children := ProcessProgram(p.Children, seq+1, bindName(bound, message), err)
					proc = append(proc, &Proc{subscribeType, channel, message, children})
				}
			}

		case ASTSend:
			if len(p.L) == 1 {
				channel := resolveName(p.R[0], bound, err)
				children := ProcessProgram(p.Children, seq, bound, err)

				if len(p.L[0]) == 0 {
					// Create temporary message channel.
					message := Var{false, seq, "<tmp>"}
					proc = append(proc, &Proc{PINewRef, message, Var{}, []*Proc{
						&Proc{PISend, channel, message, []*Proc{
							&Proc{PIPopRef, message, Var{}, children},
						}},
					}})
				} else {
					// Lookup message in bound variables.
					message := resolveName(p.L[0], bound, err)
					proc = append(proc, &Proc{PISend, channel, message, children})
				}
			}
		}
	}

	return proc
}

// Check if a name is bound or if it is an interface channel.
func resolveName(name string, bound map[string]uint, err *errorList) Var {
	// Check if the name is bound.
	if index, in := bound[name]; in {
		return Var{false, index, name}
	}
	// Hexadecimal stdin/stdout
	offset := uint(0)
	m := stdinHexRe.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutIDOffset
		m = stdoutHexRe.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		v, _ := hex.DecodeString(m[1])
		return Var{true, offset + uint(v[0]), name}
	}
	// Alphanumeric stdin/stdout
	offset = 0
	m = stdinANRe.FindStringSubmatch(name)
	if len(m) == 0 {
		offset = stdoutIDOffset
		m = stdoutANRe.FindStringSubmatch(name)
	}
	if len(m) != 0 {
		b := byte(m[1][0])
		return Var{true, offset + uint(b), name}
	}
	// Other special channels
	for k, id := range specialChannels {
		if name == k {
			return Var{true, id, name}
		}
	}
	err.Add(fmt.Errorf("unbound variable \"%v\"", name))
	// Note that Var{true, 0} is always valid.
	return Var{true, 0, name}
}

// Copy all bound variables and add a new variable.
func bindName(bound map[string]uint, v Var) map[string]uint {
	copy := copyMap(bound)
	copy[v.Name] = v.Value
	return copy
}

// Keep track of errors using this error list.
type errorList []error

func (l *errorList) Add(err error) {
	if err != nil {
		*l = append(*l, err)
	}
}
