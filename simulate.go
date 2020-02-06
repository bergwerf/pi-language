package main

import (
	"container/list"
	"fmt"
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

			if node.Proc.Type == PISubsOne || node.Proc.Type == PISubsAll {
				// Move node to subscription map.
				channel := node.Proc.Channel.ID(node)
				addSub(sub, channel, node)
			} else {
				// Move node to secondary stack.
				stack2 = append(stack2, node)
			}
		}

		// Pop secondary stack.
		if len(stack2) > 0 {
			node, stack2 = popStack(stack2)

			if node.Proc.Type == PINewRef {
				// Push new channel reference.
				assert(len(node.Refs) == int(node.Proc.Channel.Value))
				allocSequence++
				refs := copyAppend(node.Refs, allocSequence)
				for _, p := range node.Proc.Children {
					stack1 = append(stack1, Node{p, refs})
				}
			} else if node.Proc.Type == PIPopRef {
				// Pop channel reference.
				assert(len(node.Refs) == 1+int(node.Proc.Channel.Value))
				refs := node.Refs[:len(node.Refs)-1]
				for _, p := range node.Proc.Children {
					stack1 = append(stack1, Node{p, refs})
				}
			} else if node.Proc.Type == PISend {
				// Get target channel and message channel.
				channel := node.Proc.Channel.ID(node)
				message := node.Proc.Message.ID(node)

				// Check for:
				// + Standard input byte read channel.
				// + Standard output byte write channel.
				// + Debug info channel.
				if channel == specialChannels["stdin_read"] {
					// Wait for next byte (or EOF)
					buf := make([]byte, 1)
					if _, err := input.Read(buf); err == nil {
						// Push a byte channel send to the secondary stack.
						target := Var{true, uint(buf[0]), "<stdin>"}
						carrier := Var{true, message, "<carrier>"}
						trigger := &Proc{PISend, target, carrier, nil}
						stack2 = append(stack2, Node{trigger, nil})
					} else if err == io.EOF {
						// Push an EOF send.
						target := Var{true, specialChannels["stdin_EOF"], "stdin_EOF"}
						carrier := Var{true, message, "<carrier>"}
						trigger := &Proc{PISend, target, carrier, nil}
						stack2 = append(stack2, Node{trigger, nil})
					}
				} else if stdoutIDOffset <= channel && channel < stdoutIDOffset+256 {
					// Write byte to stdout.
					b := byte(channel - stdoutIDOffset)
					output.Write([]byte{b})
				} else if channel == specialChannels["debug"] {
					// Print message information.
					printDebugInfo(sub, message, node.Proc.Message)
				}

				// Push subscribed nodes on the primary stack.
				if subs, nonEmpty := sub[channel]; nonEmpty {
					for n := subs.Front(); n != nil; n = n.Next() {
						node := n.Value.(Node)
						assert(node.Proc.Channel.ID(node) == channel)
						assert(len(node.Refs) == int(node.Proc.Message.Value))

						// Add message to node references and push children on the stack.
						refs := copyAppend(node.Refs, message)
						for _, p := range node.Proc.Children {
							stack1 = append(stack1, Node{p, refs})
						}

						// Remove PISubsOne subscription.
						if node.Proc.Type == PISubsOne {
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

// Print debug information for the given variable.
func printDebugInfo(sub map[uint]*list.List, id uint, v Var) {
	fmt.Printf("--- START DEBUG INFO ---")
	fmt.Printf("name: %v\n", v.Name)
	if subs, nonEmpty := sub[id]; nonEmpty {
		fmt.Printf("subscribers:\n")
		for n := subs.Front(); n != nil; n = n.Next() {
			// Next time we need this it may be more useful to print the source
			// location at which this process is defined.
			node := n.Value.(Node)
			fmt.Printf("- node of type %v\n", node.Proc.Type)
		}
	} else {
		println("no subscribers")
	}
	fmt.Printf("--- END DEBUG INFO ---")
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
