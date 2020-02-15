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
	name       = "\\s*([a-zA-Z0-9_@]+)\\s*"
	argument   = "([\\sa-zA-Z0-9_@,]*)"
	nameCan    = "([a-zA-Z0-9_@]+)"
)

var (
	directiveRE, _ = regexp.Compile("^#(.+):([^!]*).*$")
)

// Core language
const (
	PINewRef = iota
	PISubsOne
	PISubsAll
	PISend
)

// Proc is a PI process.
type Proc struct {
	Location Loc
	Command  int
	Channel  uint    // Variable of new or receive/send channel
	Message  uint    // Variable of receive/send message
	Children []*Proc // Child processes (parallel)
}

// Core syntax.
var coreSyntax = []Transform{
	trans("\\+%v", 1, func(loc Loc, v []uint) *Proc { return &Proc{loc, PINewRef, v[0], 0, nil} }, nameCan),
	trans("%v<-%v", 1, func(loc Loc, v []uint) *Proc { return &Proc{loc, PISubsOne, v[1], v[0], nil} }, nameCan, nameCan),
	trans("%v<<%v", 1, func(loc Loc, v []uint) *Proc { return &Proc{loc, PISubsAll, v[1], v[0], nil} }, nameCan, nameCan),
	trans("%v->%v", 0, func(loc Loc, v []uint) *Proc { return &Proc{loc, PISend, v[1], v[0], nil} }, nameCan, nameCan),
}

// Rewrites to convert PI source code to a normal form. To avoid collisions
// each rewrite should use a unique new fresh variable (@n).
var extendedSyntax = []Rewrite{
	// Remove comments.
	rw("!.*", ""),

	// 1. Basic shortcuts

	// Create and send: +y->x === +y;y->x
	rw("\\+%v->%v", "+%[1]v;%[1]v->%[2]v", argument, argument),

	// Wait for trigger: <-x === @<-x
	rw("<-%v", "@1<-%[1]v", name),

	// Receive all triggers: <<x === @<<x
	rw("<<%v", "@2<<%[1]v", name),

	// Trigger once: ->x === +@;@->x
	rw("->%v", "+@3;@3->%[1]v", name),

	// Trigger and wait: <>x === +@;@->x;<-@
	rw("<>%v", "+@4;@4->%[1]v;<-@4", name),

	// Forward to channel: x>>y === @<<x;@->y
	rw("%v>>%v", "@5<<%[1]v;@5->%[2]v", name, argument),

	// 2. Variadic variants

	// Variadic create: +a,b === +a;+b
	rw("\\+%v,%v", "+%[1]v;+%[2]v", argument, argument),

	// Variadic receive: a,b<-x === a<-x;b<-x
	rw("%v,%v<-%v", "%[1]v<-%[3]v;%[2]v<-%[3]v", argument, argument, name),

	// Variadic send LHS: a,b->x === a->x;b->x
	rw("%v,%v->%v", "%[1]v->%[3]v;%[2]v->%[3]v", argument, argument, name),

	// Variadic send RHS: a->b,c === a->b;a->c
	rw("%v->%v,%v", "%[1]v->%[2]v;%[1]v->%[3]v", argument, argument, argument),

	// 3. Sending a tunnel to send or receive a stream.

	// Send through tunnel: y>->x === +@;@->x;<-@;y->@
	rw("%v>->%v", "+@6a;@6a->%[2]v;@6b<-@6a;%[1]v->@6b", argument, name),

	// Receive through tunnel: y<-<x === +@;@->x;y<-@
	rw("%v<-<%v", "+@7;@7->%[2]v;%[1]v<-@7", argument, name),

	// 4. Receiving a tunnel to send or receive a stream.

	// Tunneled receive one: y<<-x === @<-x;->@;y<-@
	rw("%v<<-%v", "@8a<-%[2]v;+@8b->@8a;%[1]v<-@8b", argument, name),

	// Tunneled receive all: y<<<x === @<<x;->@;y<-@
	rw("%v<<<%v", "@9a<<%[2]v;+@9b->@9a;%[1]v<-@9b", argument, name),
}

// IO channels
var (
	// 0..255
	stdinHexRE, _      = regexp.Compile("^stdin_([0-9A-F]{2})$")
	stdinAlphaNumRE, _ = regexp.Compile("^stdin__([a-zA-Z0-9])$")

	// 256..511
	stdoutOffset        = uint(256)
	stdoutHexRE, _      = regexp.Compile("^stdout_([0-9A-F]{2})$")
	stdoutAlphaNumRE, _ = regexp.Compile("^stdout__([a-zA-Z0-9])$")

	// Other channels
	miscIOChannels = map[string]uint{
		"stdin_EOF":  513,
		"stdin_read": 512,
		"DEBUG":      514,
	}

	ioChannelOffset = uint(515)
)

// BuildProc builds a process.
type BuildProc func(Loc, []uint) *Proc

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
