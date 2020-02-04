package main

import (
	"container/list"
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

	stdinReadName  = "stdin_read"
	stdinReadID    = uint(512)
	reservedLength = uint(513)
)

// Var represents a variable.
type Var struct {
	Raw   bool // Raw variable ID (for interface channels)
	Value uint // Variable index or ID
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
	X, Y     Var     // X and Y variables
	Children []*Proc // Child processes (parallel)
	Scope    []uint  // Allocated variables (by index) that are in use
}

// ProcessProgram processes a parsed program. It replaces variable names with
// scope indices, and returns the variables that are in use (this may be used
// in the future to clean up running processes).
func ProcessProgram(program []*Process, seq uint, bound map[string]uint) (
	[]*Proc, *list.List, []error) {
	proc := make([]*Proc, len(program))
	fullScope := list.New()
	errors := make([]error, 0)

	for i, p := range program {
		switch p.Type {
		case PIAllocate:
			// Bind X to next variable index.
			bindX := copyMap(bound)
			bindX[p.X] = seq

			c, scope, err := ProcessProgram(p.Children, seq+1, bindX)

			proc[i] = &Proc{PIAllocate, Var{false, seq}, Var{}, c, toSlice(scope)}
			ListUnion(fullScope, scope)
			errors = append(errors, err...)

		case PIReceiveOne:
			fallthrough
		case PIReceiveAll:
			// Bind Y to next variable index.
			x, xD, err1 := resolveName(p.X, bound)
			bindY := copyMap(bound)
			bindY[p.Y] = seq

			c, scope, err2 := ProcessProgram(p.Children, seq+1, bindY)
			ListUnion(scope, xD)

			proc[i] = &Proc{p.Type, x, Var{false, seq}, c, toSlice(scope)}
			ListUnion(fullScope, scope)
			errors = append(errors, mergeErr(err1, err2)...)

		case PISend:
			// Resolve both variables.
			x, xD, err1 := resolveName(p.X, bound)
			y, yD, err2 := resolveName(p.Y, bound)

			c, scope, err3 := ProcessProgram(p.Children, seq, bound)
			ListUnion(scope, xD)
			ListUnion(scope, yD)

			proc[i] = &Proc{PISend, x, y, c, toSlice(scope)}
			ListUnion(fullScope, scope)
			errors = append(errors, mergeErr(err1, mergeErr(err2, err3))...)
		}
	}

	return proc, fullScope, errors
}

// Check if a name is bound or if it is an interface channel.
func resolveName(name string, bound map[string]uint) (Var, *list.List, error) {
	// Check if the name is bound.
	if index, in := bound[name]; in {
		localScope := list.New()
		localScope.PushBack(index)
		return Var{false, index}, localScope, nil
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
		return Var{true, offset + uint(v[0])}, list.New(), nil
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
		return Var{true, offset + uint(b)}, list.New(), nil
	}
	// Control channels
	if name == stdinReadName {
		return Var{true, stdinReadID}, list.New(), nil
	}
	// Note that Var{true, 0} is always valid.
	return Var{true, 0}, list.New(), fmt.Errorf("unbound variable \"%v\"", name)
}
