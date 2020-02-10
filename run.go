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
	Seqs []uint // Message sequence of last sent message on each channel
}

// Msg is a message.
type Msg struct {
	Seq    int
	ID     uint
	Sender *Proc
}

// Channel is a channel message queue.
type Channel struct {
	Seq      int
	Messages []Msg
}

// RunProc runs the given pre-processed program.
func RunProc(proc []*Proc, input io.Reader, output io.Writer) {
	// Global channel identifier sequence.
	channels := reservedLength

	queue := make([]Node, 0)         // Scheduled nodes
	ether := make(map[uint]*Channel) // Floating messages
	sub := make(map[uint]*list.List) // Nodes waiting on a channel

	// Queue input program.
	for _, p := range proc {
		queue = append(queue, Node{p, nil, nil})
	}

	// Run simulation until no more nodes are scheduled for running time and the
	// ether is empty (all messages have been delivered).
	for len(queue)+len(ether) > 0 {
		// There are a lot of different ways to decide which process gets to run
		// next or which message is delivered next. Randomizing this might be
		// interesting for fuzzing. Trying all permutations could prove the
		// soundness of a program. And intelligent bookkeeping might allow us to
		// repress processes that are using a lot of processing time. The README
		// lists some semantics considerations. This is a fairly naive approach.

		// Run all queued nodes.
		for len(queue) > 0 {
			var node Node
			node, queue = queue[0], queue[1:]
			switch node.Proc.Type {
			case PINewRef:
				assert(len(node.Refs) == int(node.Proc.Channel.Value))
				refs := copyAppend(node.Refs, channels)
				seqs := copyAppend(node.Seqs, 0)
				channels++
				for _, p := range node.Proc.Children {
					queue = append(queue, Node{p, refs, seqs})
				}

			case PIPopRef:
				assert(len(node.Refs)-1 == int(node.Proc.Channel.Value))
				refs := node.Refs[:len(node.Refs)-1]
				seqs := node.Seqs[:len(node.Seqs)-1]
				for _, p := range node.Proc.Children {
					queue = append(queue, Node{p, refs, seqs})
				}

			case PISubsOne:
				fallthrough
			case PISubsAll:
				channel := node.Proc.Channel.ID(node)
				subscribe(sub, channel, node)

			case PISend:
				channel := node.Proc.Channel.ID(node)
				message := node.Proc.Message.ID(node)
				seqs := node.Seqs

				// Messages to the debug interface channel are handled immediately. This
				// is practical because if we wait the subscriber map may change.
				if channel == specialChannels["debug"] {
					printDebugInfo(sub, message)
				} else {
					// Otherwise send and update sequence number for this channel. We do
					// not set sequence numbers for interface channels.
					seq := send(ether, channel, message, node.Proc)
					if !node.Proc.Channel.Raw {
						seqs = make([]uint, len(node.Seqs))
						copy(seqs, node.Seqs)
						seqs[node.Proc.Channel.Value] = uint(seq)
					}
				}

				for _, p := range node.Proc.Children {
					queue = append(queue, Node{p, node.Refs, seqs})
				}
			}
		}

		// Deliver some messages.
		for channel, info := range ether {
			if len(info.Messages) == 0 {
				delete(ether, channel)
				continue
			}

			// We can clear at most one message per channel each cycle since we have
			// to let receiving processes refresh their subscription in between.
			var msg Msg
			msg, info.Messages = info.Messages[0], info.Messages[1:]
			if subs, nonEmpty := sub[channel]; nonEmpty {
				for n := subs.Front(); n != nil; n = n.Next() {
					node := n.Value.(Node)
					p := node.Proc
					assert(p.Channel.ID(node) == channel)
					assert(len(node.Refs) == int(p.Message.Value))

					// Check if this message is after the node specific channel sequence.
					// We do not do sequence checks on interface channels because you
					// should not both send and listen on those (perhaps this is a bit of
					// a silly optimization).
					if !p.Channel.Raw && uint(msg.Seq) <= node.Seqs[p.Channel.Value] {
						continue
					}

					// Append message to references and queue child processes.
					refs := copyAppend(node.Refs, msg.ID)
					seqs := copyAppend(node.Seqs, 0)
					for _, p := range p.Children {
						queue = append(queue, Node{p, refs, seqs})
					}

					// Remove PISubsOne subscription.
					if node.Proc.Type == PISubsOne {
						subs.Remove(n)
					}
				}
			}

			// Handle interface messages.
			handleInterfaceMessage(input, output, ether, channel, msg.ID)
		}
	}
}

func subscribe(sub map[uint]*list.List, channel uint, node Node) {
	if subs, in := sub[channel]; in {
		subs.PushBack(node)
	} else {
		subs := list.New()
		subs.PushBack(node)
		sub[channel] = subs
	}
}

func send(ether map[uint]*Channel, channel uint, message uint, s *Proc) int {
	if c, exists := ether[channel]; exists {
		c.Seq++
		c.Messages = append(c.Messages, Msg{c.Seq, message, s})
		return c.Seq
	}
	ether[channel] = &Channel{1, []Msg{Msg{1, message, s}}}
	return 1
}

func handleInterfaceMessage(
	input io.Reader, output io.Writer,
	ether map[uint]*Channel, channel uint, message uint) {
	// + Standard input read trigger.
	// + Standard output byte trigger.
	// + Debug info channel.
	if channel == specialChannels["stdin_read"] {
		// Wait for next byte (or EOF)
		buf := make([]byte, 1)
		if _, err := input.Read(buf); err == nil {
			// Send byte read trigger.
			send(ether, uint(buf[0]), message, nil)
		} else if err == io.EOF {
			// Send EOF trigger.
			send(ether, specialChannels["stdin_EOF"], message, nil)
		}
	} else if stdoutIDOffset <= channel && channel < stdoutIDOffset+256 {
		// Write byte to stdout and send write trigger.
		b := byte(channel - stdoutIDOffset)
		output.Write([]byte{b})
		send(ether, specialChannels["stdout_write"], message, nil)
	}
}

// Print subscribers of a channel.
func printDebugInfo(sub map[uint]*list.List, channel uint) {
	println()
	println("--- DEBUG SECTION ---")
	fmt.Printf("channel: %v\n", channel)
	if subs, nonEmpty := sub[channel]; nonEmpty && subs.Len() > 0 {
		for n := subs.Front(); n != nil; n = n.Next() {
			node := n.Value.(Node)
			fmt.Printf("+ %v\n", node.Proc.Loc)
		}
	}
	println("---------------------")
}
