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

	stack1 := make([]Node, 0)        // Primary stack
	stack2 := make([]Node, 0)        // Secondary stack (allocate and sent)
	sub := make(map[uint]*list.List) // Subscribers

	// Queue the input program.
	for _, p := range proc {
		stack1 = append(stack1, Node{p, nil})
	}

	// Run simulation until all processes are finished (the whole stack is empty).
	for len(stack1)+len(stack2) > 0 {
		var node Node

		// Empty the primary stack.
		for len(stack1) > 0 {
			node, stack1 = popStack(stack1)

			if node.Proc.Type == PIReceiveOne || node.Proc.Type == PIReceiveAll {
				// Move node to subscription map.
				assert(len(node.Refs) == int(node.Proc.Y.Value))
				channel := node.Proc.X.ID(node)
				addSub(sub, channel, node)
			} else {
				// Move node to secondary stack.
				stack2 = append(stack2, node)
			}
		}

		// Pop secondary stack.
		if len(stack2) > 0 {
			node, stack2 = popStack(stack2)

			if node.Proc.Type == PIAllocate {
				// Allocate new channel ID and push child nodes on the primary stack.
				assert(len(node.Refs) == int(node.Proc.X.Value))
				allocSequence++
				refs := append(node.Refs, allocSequence)
				for _, p := range node.Proc.Children {
					stack1 = append(stack1, Node{p, refs})
				}
			} else if node.Proc.Type == PISend {
				// Get target channel and message channel.
				channel := node.Proc.X.ID(node)
				message := node.Proc.Y.ID(node)

				// Check if this is an interface channel.
				if channel == stdinReadID {
					// Try to read next byte.
					if b, err := bufin.ReadByte(); err == nil {
						// Push a byte channel send to the secondary stack.
						target := Var{true, uint(b)}
						carrier := Var{true, message}
						trigger := &Proc{PISend, target, carrier, nil, nil}
						stack2 = append(stack2, Node{trigger, nil})
					}
				} else if stdoutIDOffset <= channel && channel < stdinReadID {
					// Write byte to stdout.
					b := byte(channel - stdoutIDOffset)
					output.Write([]byte{b})
				}

				// Push subscribed nodes on the primary stack.
				if subs, nonEmpty := sub[channel]; nonEmpty {
					for n := subs.Front(); n != nil; n = n.Next() {
						node := n.Value.(Node)
						assert(node.Proc.X.ID(node) == channel)
						assert(len(node.Refs) == int(node.Proc.Y.Value))

						// Add message to node references and push children on the stack.
						refs := append(node.Refs, message)
						for _, p := range node.Proc.Children {
							stack1 = append(stack1, Node{p, refs})
						}

						// Remove PIReceiveOne subscription.
						if node.Proc.Type == PIReceiveOne {
							subs.Remove(n)
						}
					}
				}

				// Push child nodes on the primary stack.
				for _, p := range node.Proc.Children {
					stack1 = append(stack1, Node{p, node.Refs})
				}
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

func popStack(stack []Node) (Node, []Node) {
	return stack[len(stack)-1], stack[:len(stack)-1]
}
