package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	var stdin io.Reader
	stdin = os.Stdin

	// Parse all files given by the command line arguments.
	program := make([]*Process, 0)
	for _, name := range os.Args[1:] {
		// An --stdin flag is supported for debugging.
		if strings.HasPrefix(name, "--stdin=") {
			stdin = strings.NewReader(name[8:])
			continue
		}

		// Try to read.
		bytes, ioErr := ioutil.ReadFile(name)
		if ioErr != nil {
			panic(ioErr)
		}

		// Try to parse.
		proc, parseErr := Parse(string(bytes))
		program = append(program, proc...)
		if len(parseErr) != 0 {
			for _, e := range parseErr {
				println(e.Error())
			}
			panic(fmt.Sprintf("Terminated because \"%v\" contains errors.", name))
		}
	}

	// Process program.
	err := errorList([]error{})
	proc := ProcessProgram(program, uint(0), nil, &err)
	if len(err) != 0 {
		for _, e := range err {
			println(e.Error())
		}
	}

	// Simulate program.
	Simulate(proc, stdin, os.Stdout)
}
