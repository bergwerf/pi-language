package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var stdin io.Reader
	stdin = os.Stdin

	// Parse all files given by the command line arguments.
	program := make([]*Process, 0)
	stack := make([]string, 0) // File reading stack
	global := MakeSet()        // Global names
	loaded := MakeSet()        // Already parsed files

	for _, arg := range os.Args[1:] {
		// An --stdin flag is supported for debugging.
		if strings.HasPrefix(arg, "--stdin=") {
			stdin = strings.NewReader(arg[8:])
		} else {
			path, _ := filepath.Abs(arg)
			stack = append(stack, path)
		}
	}

	for len(stack) > 0 {
		var path string
		path, stack = stack[len(stack)-1], stack[:len(stack)-1]
		if loaded.Contains(path) {
			continue
		}
		loaded.Add(path)

		// Try to read file.
		bytes, ioErr := ioutil.ReadFile(path)
		if ioErr != nil {
			panic(ioErr)
		}

		// Extract directives.
		attachAdd, globalAdd, offset, source := ExtractDirectives(string(bytes))
		global.AddAll(globalAdd)

		// Add attached files relative to this file.
		for _, attachment := range attachAdd {
			abs, _ := filepath.Abs(filepath.Join(filepath.Dir(path), attachment))
			stack = append(stack, abs)
		}

		// Try to parse.
		proc, parseErr := Parse(source, Loc{path, offset + 1, 0})
		program = append(program, proc...)
		if len(parseErr) != 0 {
			for _, e := range parseErr {
				println(e.Error())
			}
			panic(fmt.Sprintf("Terminated because \"%v\" contains errors.", path))
		}
	}

	// Wrap all processes in globally defined names.
	if len(global) > 0 {
		program = []*Process{
			&Process{Loc{}, ASTCreate, nil, global.ToSlice(), program}}
	}

	// Process program.
	err := errorList([]error{})
	proc := ProcessProgram(program, uint(0), nil, &err)
	if len(err) != 0 {
		for _, e := range err {
			println(e.Error())
		}
	} else {
		// Run program.
		RunProc(proc, stdin, os.Stdout)
	}
}
