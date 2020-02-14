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
	Seq       uint
	ChannelID uint
	MessageID uint
}

// Channel holds channel and subscription information.
type Channel struct {
	Seq       uint
	Listening *list.List
	PrevCycle uint64
}

// PrintListeners of a channel.
func (c *Channel) PrintListeners() {
	for n := c.Listening.Front(); n != nil; n = n.Next() {
		node := n.Value.(Node)
		fmt.Printf("+ %v\n", node.Proc.Location)
	}
}

// RunProc runs the given pre-processed program.
func RunProc(proc []*Proc, input io.Reader, output io.Writer) {
	queue := make([]Node, 0)
	ether := make([]Msg, 0)
	network := make(map[uint]*Channel)

	channelSeq := uint(0) // Used to number channels.
	cycle := uint64(0)    // Deliver <=1 message per cycle per channel.

	// Add all interface channels to the network.
	for channelSeq < reservedChannelIDs {
		network[channelSeq] = &Channel{0, list.New(), 0}
		channelSeq++
	}

	// Queue input program.
	for _, p := range proc {
		queue = append(queue, Node{p, nil, nil})
	}

	// Run simulation until no more nodes are scheduled for running time and the
	// ether is empty (all messages have been delivered).
	for len(queue)+len(ether) > 0 {
		// Run all queued nodes.
		for len(queue) > 0 {
			var node Node
			node, queue = queue[0], queue[1:]
			switch node.Proc.Action {
			case PINewRef:
				assert(len(node.Refs) == int(node.Proc.Channel.Value))
				refs := copyAppend(node.Refs, channelSeq)
				seqs := copyAppend(node.Seqs, 0)
				network[channelSeq] = &Channel{0, list.New(), 0}
				channelSeq++
				for _, p := range node.Proc.Children {
					queue = append(queue, Node{p, refs, seqs})
				}

			case PISubsOne:
				fallthrough
			case PISubsAll:
				channelID := node.Proc.Channel.ID(node)
				network[channelID].Listening.PushBack(node)

			case PISend:
				channelID := node.Proc.Channel.ID(node)
				messageID := node.Proc.Message.ID(node)
				channel := network[channelID]
				seqs := node.Seqs

				// Messages to the debug interface channel are handled immediately. This
				// is practical because if we wait the subscriber map may change.
				if channelID == specialChannels["DEBUG"] {
					println()
					println("--- DEBUG SECTION ---")
					fmt.Printf("channel: %v\n", channelID)
					channel.PrintListeners()
					println("---------------------")
				} else {
					// Otherwise send and update sequence number for this channel. We do
					// not set sequence numbers for interface channels.
					channel.Seq++
					ether = append(ether, Msg{channel.Seq, channelID, messageID})
					if !node.Proc.Channel.Raw {
						seqs = make([]uint, len(node.Seqs))
						copy(seqs, node.Seqs)
						seqs[node.Proc.Channel.Value] = channel.Seq
					}
				}

				for _, p := range node.Proc.Children {
					queue = append(queue, Node{p, node.Refs, seqs})
				}
			}
		}

		// Deliver at most one message per channel.
		cycle++
		messages := ether
		ether = ether[0:0]
		for _, msg := range messages {
			// Check if we already delivered a message to this channel in this cycle.
			channel := network[msg.ChannelID]
			if channel.PrevCycle == cycle {
				ether = append(ether, msg)
				continue
			}
			channel.PrevCycle = cycle

			for ptr := channel.Listening.Front(); ptr != nil; ptr = ptr.Next() {
				node := ptr.Value.(Node)
				p := node.Proc
				assert(p.Channel.ID(node) == msg.ChannelID)
				assert(len(node.Refs) == int(p.Message.Value))

				// Check if this message is after the node specific channel sequence.
				if !p.Channel.Raw && msg.Seq <= node.Seqs[p.Channel.Value] {
					continue
				}

				// Append message to references and queue child processes.
				refs := copyAppend(node.Refs, msg.MessageID)
				seqs := copyAppend(node.Seqs, 0)
				for _, p := range p.Children {
					queue = append(queue, Node{p, refs, seqs})
				}

				// Remove PISubsOne subscription.
				if node.Proc.Action == PISubsOne {
					channel.Listening.Remove(ptr)
				}
			}

			// Handle interface messages. Note that the way we iterate and overwrite
			// the ether buffer at the same time is only ok as long as this function
			// returns at most one message.
			ether = append(ether, handleInterfaceMessage(input, output, msg)...)
		}
	}
}

func handleInterfaceMessage(input io.Reader, output io.Writer, msg Msg) []Msg {
	// + Standard input read trigger.
	// + Standard output byte trigger.
	// + Debug info channel.
	id := msg.ChannelID
	if id == specialChannels["stdin_read"] {
		// Wait for next byte (or EOF)
		buf := make([]byte, 1)
		if _, err := input.Read(buf); err == nil {
			// Send byte read trigger.
			return []Msg{Msg{0, uint(buf[0]), msg.MessageID}}
		} else if err == io.EOF {
			// Send EOF trigger.
			return []Msg{Msg{0, specialChannels["stdin_EOF"], msg.MessageID}}
		}
	} else if stdoutIDOffset <= id && id < stdoutIDOffset+256 {
		// Write byte to stdout and send write trigger.
		b := byte(id - stdoutIDOffset)
		output.Write([]byte{b})
		return []Msg{Msg{0, specialChannels["stdout_write"], msg.MessageID}}
	}
	return nil
}
