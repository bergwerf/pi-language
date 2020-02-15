package main

import (
	"fmt"
	"io"
)

// Pi represents the state of a Pi program.
type Pi struct {
	Cycle uint64
	Queue []Node
	Ether []Message
	Stdio []*Channel
}

// Channel holds channel and subscription information.
type Channel struct {
	IOIndex   int    // -1 or IO channel index
	Listeners []Node // Current channel listeners
	PrevCycle uint64 // Previous cycle in which a message was delivered
}

// Node represents a process with a number of bound channels. This follows the
// signalling network metaphor. A process is a pointer to the program AST.
type Node struct {
	Proc *Proc      // Process at which this node is paused
	Refs []*Channel // Referenced channels
}

// Message represents a single message.
type Message struct {
	Channel *Channel
	Content *Channel
}

// Schedule adds child processes to the queue with the provided references.
func (pi *Pi) Schedule(proc []*Proc, refs []*Channel) {
	// Make sure each process gets a copy of the references such that they can
	// modify the references in place without interferring with other processes.
	for i, p := range proc {
		if i == 0 {
			pi.Queue = append(pi.Queue, Node{p, refs})
		} else {
			pi.Queue = append(pi.Queue, Node{p, copyRefs(refs)})
		}
	}
}

// Initialize sets up the initial program state.
func (pi *Pi) Initialize(proc []*Proc) {
	// Create IO channels.
	pi.Stdio = make([]*Channel, ioChannelOffset)
	for i := 0; i < int(ioChannelOffset); i++ {
		pi.Stdio[i] = &Channel{i, nil, 0}
	}
	pi.Schedule(proc, copyRefs(pi.Stdio))
}

// RunNextNode executes the top node in the process queue.
func (pi *Pi) RunNextNode() {
	if len(pi.Queue) == 0 {
		return
	}

	var node Node
	node, pi.Queue = pi.Queue[0], pi.Queue[1:]
	switch node.Proc.Command {
	case PINewRef:
		assert(len(node.Refs) == int(node.Proc.Channel))
		refs := append(node.Refs, &Channel{-1, nil, 0})
		pi.Schedule(node.Proc.Children, refs)

	case PISubsOne:
		fallthrough
	case PISubsAll:
		channel := node.Refs[node.Proc.Channel]
		channel.Listeners = append(channel.Listeners, node)

	case PISend:
		channel := node.Refs[node.Proc.Channel]
		message := node.Refs[node.Proc.Message]
		pi.Ether = append(pi.Ether, Message{channel, message})
		pi.Schedule(node.Proc.Children, node.Refs)

		// Messages to the debug channel are handled immediately. This is practical
		// because if we wait the listeners may change.
		if channel.IOIndex == int(miscIOChannels["DEBUG"]) {
			message.PrintDebugInfo()
		}
	}
}

// DeliverMessages delivers up to one message per channel from the ether.
func (pi *Pi) DeliverMessages(input io.Reader, output io.Writer) {
	pi.Cycle++
	messages := pi.Ether
	pi.Ether = pi.Ether[0:0]

	for _, m := range messages {
		// Check if we can send a message on this channel in the current cycle. We
		// send only one message per channel per cycle!
		if m.Channel.PrevCycle < pi.Cycle {
			m.Channel.PrevCycle = pi.Cycle
		} else {
			// Put message back into the ether.
			pi.Ether = append(pi.Ether, m)
			continue
		}

		listeners := m.Channel.Listeners
		m.Channel.Listeners = m.Channel.Listeners[0:0]
		for _, node := range listeners {
			assert(len(node.Refs) == int(node.Proc.Message))

			// Copy references of a PISubsAll subscription and renew subscription.
			refs := node.Refs
			if node.Proc.Command == PISubsAll {
				refs = copyRefs(node.Refs)
				m.Channel.Listeners = append(m.Channel.Listeners, node)
			}

			// Append message content to references and queue child processes.
			refs = append(refs, m.Content)
			pi.Schedule(node.Proc.Children, refs)
		}

		// Handle IO messages. Note that the way we iterate and overwrite the ether
		// buffer at the same time is only ok as long as this function returns at
		// most one message.
		if m.Channel.IOIndex != -1 {
			ioMessages := handleStdioMessage(pi.Stdio, input, output, m)
			pi.Ether = append(pi.Ether, ioMessages...)
		}
	}
}

func handleStdioMessage(stdio []*Channel, in io.Reader, out io.Writer, m Message) []Message {
	// + Standard input read trigger.
	// + Standard output byte trigger.
	// + Debug info channel.
	id := uint(m.Channel.IOIndex)
	if id == miscIOChannels["stdin_read"] {
		// Wait for next byte (or EOF)
		buf := make([]byte, 1)
		if _, err := in.Read(buf); err == nil {
			// Send byte read trigger.
			byteReadChannel := stdio[buf[0]]
			return []Message{Message{byteReadChannel, m.Content}}
		} else if err == io.EOF {
			// Send EOF trigger.
			eofChannel := stdio[miscIOChannels["stdin_EOF"]]
			return []Message{Message{eofChannel, m.Content}}
		}
	} else if stdoutOffset <= id && id < stdoutOffset+256 {
		// Write byte to stdout and send acknowledgement message.
		b := byte(id - stdoutOffset)
		out.Write([]byte{b})
		return []Message{Message{m.Content, m.Content}}
	}
	return nil
}

// PrintDebugInfo prints the listeners.
func (c *Channel) PrintDebugInfo() {
	println()
	println("--- DEBUG SECTION ---")
	fmt.Printf("channel address: %p\n", c)
	for _, node := range c.Listeners {
		fmt.Printf("+ %v\n", node.Proc.Location)
	}
	println("---------------------")
}

func copyRefs(src []*Channel) []*Channel {
	// 7 is arbitrary, but we add a bit of extra capacity to future appends.
	dst := make([]*Channel, len(src), len(src)+7)
	copy(dst, src)
	return dst
}
