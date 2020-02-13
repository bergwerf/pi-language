package main

import (
	"fmt"
	"regexp"
)

// Language primitives
const (
	sComment   = "!"
	sParOpen   = "("
	sParClose  = ")"
	sSemicolon = ";"
	sPeriod    = "."
	control    = "[(;.)]"
	name       = "\\s*([a-zA-Z0-9_]+)\\s*"
	argument   = "([\\sa-zA-Z0-9_,]*)"
	nameCan    = "([a-zA-Z0-9_@]+)"
)

var (
	directiveRE, _ = regexp.Compile("^#(.+):([^!]*).*$")
)

// Core language
const (
	PINewRef = iota
	PIPopRef
	PISubsOne
	PISubsAll
	PISend
)

// Proc is a PI process.
type Proc struct {
	Location Loc
	Action   int
	Channel  Var     // Variable of new/popped or receive/send channel
	Message  Var     // Variable of receive/send message
	Children []*Proc // Child processes (parallel)
}

// Core syntax.
var coreSyntax = []Transform{
	trans("\\+%v", 1, func(loc Loc, v []Var) *Proc { return &Proc{loc, PINewRef, v[0], Var{}, nil} }, nameCan),
	trans("\\~%v", 0, func(loc Loc, v []Var) *Proc { return &Proc{loc, PIPopRef, v[0], Var{}, nil} }, nameCan),
	trans("%v<-%v", 1, func(loc Loc, v []Var) *Proc { return &Proc{loc, PISubsOne, v[1], v[0], nil} }, nameCan, nameCan),
	trans("%v<<%v", 1, func(loc Loc, v []Var) *Proc { return &Proc{loc, PISubsAll, v[1], v[0], nil} }, nameCan, nameCan),
	trans("%v->%v", 0, func(loc Loc, v []Var) *Proc { return &Proc{loc, PISend, v[1], v[0], nil} }, nameCan, nameCan),
}

// Rewrites to convert PI source code to a normal form.
var extendedSyntax = []Rewrite{
	// Remove comments.
	rw("!.*", ""),

	// Variadic create: +a,b === +a;+b
	rw("\\+%v,%v", "+%[1]v;+%[2]v", argument, argument),

	// Variadic receive: a,b<-x === a<-x;b<-x
	rw("%v,%v<-%v", "%[1]v<-%[3]v;%[2]v<-%[3]v", argument, argument, name),

	// Variadic send LHS: a,b->x === a->x;b->x
	rw("%v,%v->%v", "%[1]v->%[3]v;%[2]v->%[3]v", argument, argument, name),

	// Variadic send RHS: a->b,c === a->b;a->c
	rw("%v->%v,%v", "%[1]v->%[2]v;%[1]v->%[3]v", argument, argument, argument),

	// Wait for trigger: <-x === @<-x;~@
	rw("<-%v", "@<-%[1]v;~@", name),

	// Receive all triggers: <<x === @<<x;~@
	rw("<<%v", "@<<%[1]v;~@", name),

	// Trigger once: ->x === +@;@->x;~@
	rw("->%v", "+@;@->%[1]v;~@", name),

	// Trigger and wait: <>x === +@;@->x;<-@;~@
	rw("<>%v", "+@;@->%[1]v;<-@;~@", name),

	// Tunnel arguments: y=>x === +@;@->x;<-@;y->@;~@
	rw("%v=>%v", "+@;@->%[2]v;<-@;%[1]v->@;~@", argument, name),

	// Forward channel: x>>y === @<<x;@->y;~@
	rw("%v>>%v", "@<<%[1]v;@->%[2]v;~@", name, argument),
}

// Interface channels
var (
	// 0..255
	stdinHexRE, _      = regexp.Compile("^stdin_([0-9A-F]{2})$")
	stdinAlphaNumRE, _ = regexp.Compile("^stdin__([a-zA-Z0-9])$")

	// 256..511
	stdoutIDOffset      = uint(256)
	stdoutHexRE, _      = regexp.Compile("^stdout_([0-9A-F]{2})$")
	stdoutAlphaNumRE, _ = regexp.Compile("^stdout__([a-zA-Z0-9])$")

	// Other channels
	specialChannels = map[string]uint{
		"stdin_read":   uint(512),
		"stdin_EOF":    uint(513),
		"stdout_write": uint(514),
		"debug":        uint(515),
	}

	reservedChannelIDs = uint(516)
)

// BuildProc builds a process.
type BuildProc func(Loc, []Var) *Proc

// Rewrite is a regular expression based string rewrite.
type Rewrite struct {
	Pattern *regexp.Regexp
	Replace string
}

// Transform is a regular expression based AST transformation.
type Transform struct {
	Pattern *regexp.Regexp
	BindVar int
	Process BuildProc
}

// Create a new rewrite.
func rw(format string, output string, types ...interface{}) Rewrite {
	prefixFmt := fmt.Sprintf("^%v", format)
	typedFmt := fmt.Sprintf(prefixFmt, types...)
	re, _ := regexp.Compile(typedFmt)
	return Rewrite{re, output}
}

// Create a new transform.
func trans(format string, bind int, build BuildProc, types ...interface{}) Transform {
	prefixFmt := fmt.Sprintf("^%v$", format)
	typedFmt := fmt.Sprintf(prefixFmt, types...)
	re, _ := regexp.Compile(typedFmt)
	return Transform{re, bind, build}
}

// Var represents a variable. Raw variables are used to represent unbounded
// interface channels (input/output). This is an optimization.
type Var struct {
	Raw   bool
	Value uint
	Name  string
}

// ID returns the ID this variable is bound to on the given node.
func (v Var) ID(node Node) uint {
	if v.Raw {
		return v.Value
	}
	return node.Refs[v.Value]
}
