package main

import (
	"fmt"
	"io"
)

// Pi represents the state of a Pi program.
type Pi struct {
	Queue        []Node
	Ether        []Message
	Network      map[uint]*Channel
	CycleCount   uint64
	ChannelCount uint
}

// Node represents a process with a number of bound channels. This follows the
// signalling network metaphor. A process is a pointer from the program AST.
type Node struct {
	Proc *Proc  // Process at which this node is paused
	Refs []uint // Allocated variable IDs (including out of scope ones)
	Seqs []uint // Sequence number of last sent message on each channel
}

// Message represents a single message.
type Message struct {
	Seq       uint
	ChannelID uint
	ContentID uint
}

// Channel holds channel and subscription information.
type Channel struct {
	Seq       uint
	Listeners []Node
	PrevCycle uint64
}

// Schedule adds processes to the queue with the provided context.
func (pi *Pi) Schedule(proc []*Proc, refs []uint, seqs []uint) {
	for _, p := range proc {
		pi.Queue = append(pi.Queue, Node{p, refs, seqs})
	}
}

// Initialize sets up the initial program state.
func (pi *Pi) Initialize(proc []*Proc) {
	pi.Schedule(proc, nil, nil)

	// Add all interface channels to the network.
	for pi.ChannelCount < reservedChannelIDs {
		pi.Network[pi.ChannelCount] = &Channel{0, nil, 0}
		pi.ChannelCount++
	}
}

// RunNextNode executes the top node in the process queue.
func (pi *Pi) RunNextNode() {
	if len(pi.Queue) == 0 {
		return
	}

	var node Node
	node, pi.Queue = pi.Queue[0], pi.Queue[1:]
	switch node.Proc.Action {
	case PINewRef:
		//assert(len(node.Refs) == int(node.Proc.Channel.Value))
		id := pi.ChannelCount
		pi.ChannelCount++
		pi.Network[id] = &Channel{0, nil, 0}

		refs := copyAppend(node.Refs, id)
		seqs := copyAppend(node.Seqs, 0)
		pi.Schedule(node.Proc.Children, refs, seqs)

	case PISubsOne:
		fallthrough
	case PISubsAll:
		channelID := node.Proc.Channel.ID(node)
		channel := pi.Network[channelID]
		channel.Listeners = append(channel.Listeners, node)

	case PISend:
		channelID := node.Proc.Channel.ID(node)
		messageID := node.Proc.Message.ID(node)
		seqs := node.Seqs

		// Messages to the debug interface channel are handled immediately. This
		// is practical because if we wait the subscriber map may change.
		if channelID == specialChannels["DEBUG"] {
			printDebugInfo(messageID, pi.Network[messageID])
		} else {
			// Otherwise send and update sequence number for this channel. We do
			// not set sequence numbers for interface channels.
			channel := pi.Network[channelID]
			channel.Seq++
			pi.Ether = append(pi.Ether, Message{channel.Seq, channelID, messageID})
			if !node.Proc.Channel.Raw {
				// Deep copy the slice because the memory may be shared with
				seqs = make([]uint, len(node.Seqs))
				copy(seqs, node.Seqs)
				seqs[node.Proc.Channel.Value] = channel.Seq
			}
		}

		pi.Schedule(node.Proc.Children, node.Refs, seqs)
	}
}

// DeliverMessages delivers up to one message per channel from the ether.
func (pi *Pi) DeliverMessages(input io.Reader, output io.Writer) {
	pi.CycleCount++
	messages := pi.Ether
	pi.Ether = pi.Ether[0:0]

	for _, m := range messages {
		channel := pi.Network[m.ChannelID]

		// Check if we already delivered a message to this channel in this cycle.
		if channel.PrevCycle == pi.CycleCount {
			pi.Ether = append(pi.Ether, m)
			continue
		}

		channel.PrevCycle = pi.CycleCount
		listeners := channel.Listeners
		channel.Listeners = channel.Listeners[0:0]
		for _, n := range listeners {
			//assert(n.Proc.Channel.ID(n) == m.ChannelID)
			//assert(len(n.Refs) == int(n.Proc.Message.Value))

			// Check if this message is after the node specific channel sequence. If
			// not put the listener back.
			if !n.Proc.Channel.Raw && m.Seq <= n.Seqs[n.Proc.Channel.Value] {
				channel.Listeners = append(channel.Listeners, n)
				continue
			}

			// Append message content to references and queue child processes.
			refs := copyAppend(n.Refs, m.ContentID)
			seqs := copyAppend(n.Seqs, 0)
			pi.Schedule(n.Proc.Children, refs, seqs)

			// Renew PISubsAll subscription.
			if n.Proc.Action == PISubsAll {
				channel.Listeners = append(channel.Listeners, n)
			}
		}

		// Handle interface messages. Note that the way we iterate and overwrite
		// the ether buffer at the same time is only ok as long as this function
		// returns at most one message.
		pi.Ether = append(pi.Ether, handleInterfaceMessage(input, output, m)...)
	}
}

func handleInterfaceMessage(in io.Reader, out io.Writer, m Message) []Message {
	// + Standard input read trigger.
	// + Standard output byte trigger.
	// + Debug info channel.
	id := m.ChannelID
	if id == specialChannels["stdin_read"] {
		// Wait for next byte (or EOF)
		buf := make([]byte, 1)
		if _, err := in.Read(buf); err == nil {
			// Send byte read trigger.
			return []Message{Message{0, uint(buf[0]), m.ContentID}}
		} else if err == io.EOF {
			// Send EOF trigger.
			return []Message{Message{0, specialChannels["stdin_EOF"], m.ContentID}}
		}
	} else if stdoutIDOffset <= id && id < stdoutIDOffset+256 {
		// Write byte to stdout and send write trigger.
		b := byte(id - stdoutIDOffset)
		out.Write([]byte{b})
		return []Message{Message{0, specialChannels["stdout_write"], m.ContentID}}
	}
	return nil
}

func printDebugInfo(channelID uint, channel *Channel) {
	println()
	println("--- DEBUG SECTION ---")
	fmt.Printf("channel: %v\n", channelID)
	for _, node := range channel.Listeners {
		fmt.Printf("+ %v\n", node.Proc.Location)
	}
	println("---------------------")
}
