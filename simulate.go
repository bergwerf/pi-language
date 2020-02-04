package main

import (
	"bufio"
	"container/list"
	"io"
)

// Node represents a process with a number of bound channels. This follows the
// signalling network metaphor. A process is a pointer from the program AST.
type Node struct {
	Proc *Proc  // Process at which this node is paused
	Refs []uint // Allocated variable IDs (including out of scope ones)
}

// Simulate the given pre-processed program.
func Simulate(proc []*Proc, input io.Reader, output io.Writer) {
	bufin := bufio.NewReader(input)

	// Global channel identifier sequence.
	allocSequence := reservedLength

	// Stack of scheduled processes and map of subscriptions.
	stack := make([]Node, 0, len(proc))
	sub := make(map[uint]*list.List)

	// Queue the input program.
	for _, p := range proc {
		stack = append(stack, Node{p, nil})
	}

	// Run simulation until all processes are finished (the stack is empty).
	for len(stack) > 0 {
		// Remove node from stack.
		var node Node
		node, stack = stack[len(stack)-1], stack[:len(stack)-1]

		switch node.Proc.Type {
		case PIAllocate:
			// Allocate new channel ID and push child nodes on the stack.
			assert(len(node.Refs) == int(node.Proc.X.Value))
			allocSequence++
			refs := append(node.Refs, allocSequence)
			for _, p := range node.Proc.Children {
				stack = append(stack, Node{p, refs})
			}

		case PIReceiveOne:
			fallthrough
		case PIReceiveAll:
			// Move node to subscription map (effectively pausing it).
			assert(len(node.Refs) == int(node.Proc.Y.Value))
			channel := node.Proc.X.ID(node)
			addSub(sub, channel, node)

		case PISend:
			channel := node.Proc.X.ID(node)
			message := node.Proc.Y.ID(node)

			// Check for interface channels.
			if channel == stdinReadID {
				// Try to read next byte.
				if b, err := bufin.ReadByte(); err == nil {
					// Push a byte channel send to the stack.
					target := Var{true, uint(b)}
					carrier := Var{true, message}
					trigger := &Proc{PISend, target, carrier, nil, nil}
					stack = append(stack, Node{trigger, nil})
				}
			} else if stdoutIDOffset <= channel && channel < stdinReadID {
				// Write byte to stdout.
				b := byte(channel - stdoutIDOffset)
				output.Write([]byte{b})
			}

			// Push subscribed nodes on the stack.
			//
			// TODO: This eager strategy does not work well with parallelism. Consider
			// the following example: `y<-x;z<-y;x->z. a->x;b->a;c<-b.`
			if subs, nonEmpty := sub[channel]; nonEmpty {
				for n := subs.Front(); n != nil; n = n.Next() {
					node := n.Value.(Node)
					assert(node.Proc.X.ID(node) == channel)
					assert(len(node.Refs) == int(node.Proc.Y.Value))

					// Add message to node references and push children on the stack.
					refs := append(node.Refs, message)
					for _, p := range node.Proc.Children {
						stack = append(stack, Node{p, refs})
					}

					// Remove ASTReceiveOne subscription.
					if node.Proc.Type == PIReceiveOne {
						subs.Remove(n)
					}
				}
			}

			// Push child nodes on the stack.
			for _, p := range node.Proc.Children {
				stack = append(stack, Node{p, node.Refs})
			}
		}
	}
}

// Add key and value to subscriber multimap.
func addSub(sub map[uint]*list.List, channel uint, node Node) {
	if subs, in := sub[channel]; in {
		subs.PushBack(node)
	} else {
		subs := list.New()
		subs.PushBack(node)
		sub[channel] = subs
	}
}
