package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	// Parse all files given by the command line arguments.
	all := make([]Process, 0)
	for _, name := range os.Args[1:] {
		// Try to read.
		bytes, err := ioutil.ReadFile(name)
		if err != nil {
			panic(err)
		}

		// Try to parse.
		proc, errors := Parse(string(bytes))
		all = append(all, proc...)
		if len(errors) > 0 {
			for _, err := range errors {
				println(err.Error())
			}
			panic(fmt.Sprintf("Terminated because \"%v\" contains errors.", name))
		}
	}
}
