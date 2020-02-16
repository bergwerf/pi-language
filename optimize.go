package main

// ProcInfo contains information about a list of processes. It contains a set of
// all reference indices that are used, and an info object for the children of
// each process. This structure avoids duplicate analyses.
type ProcInfo struct {
	Proc []*Proc
	Used Set
	Info []ProcInfo
}

// Analyze computes optimization information about the given processes.
func Analyze(proc []*Proc) ProcInfo {
	used := MakeSet()
	info := make([]ProcInfo, len(proc))
	for i, p := range proc {
		// Mark process references as used.
		for _, v := range []int{p.Channel, p.Message} {
			if v != -1 {
				used.Add(v)
			}
		}
		// Analyze children.
		info[i] = Analyze(p.Children)
		used.Union(info[i].Used)
	}
	return ProcInfo{proc, used, info}
}

// Optimize inserts PIDeref commands to deference unused channels.
func Optimize(program []*Proc) []*Proc {
	// Analyze program and generate initial IO references.
	info := Analyze(program)
	refs := make([]int, ioChannelOffset)
	for i := 0; i < ioChannelOffset; i++ {
		refs[i] = i
	}
	return optimize(info, refs, ioChannelOffset)
}

func optimize(info ProcInfo, refs []int, refSeq int) []*Proc {
	// Do not dereference when there are not child processes.
	if len(info.Proc) == 0 {
		return nil
	}

	// Determine which indices in refs are not used in any of the child processes.
	deref := []int{}
	for i := 0; i < len(refs); i++ {
		if !info.Used.Contains(refs[i]) {
			deref = append(deref, i)
			refs = append(refs[:i], refs[i+1:]...)
			i--
		}
	}
	// Rebuild child processes with new refs slice.
	children := make([]*Proc, len(info.Proc))
	for i, p := range info.Proc {
		pRefs := append(refs[:0:0], refs...)
		pRefSeq := refSeq
		// Add new references to the refs slice (note that we need refSeq to compute
		// the reference index in the unoptimized program).
		if p.Command&(PINewRef|PISubsOne|PISubsAll) != 0 {
			pRefs = append(pRefs, pRefSeq)
			pRefSeq++
		}
		// Create new process node.
		children[i] = &Proc{p.Location, p.Command,
			lookupRef(p.Channel, pRefs),
			lookupRef(p.Message, pRefs),
			optimize(info.Info[i], pRefs, pRefSeq),
		}
	}
	// Prepend dereference nodes.
	proc := children
	for i := len(deref) - 1; i >= 0; i-- {
		proc = []*Proc{&Proc{Loc{}, PIDeref, deref[i], -1, proc}}
	}
	return proc
}

func lookupRef(ref int, refs []int) int {
	if ref == -1 {
		return -1
	}
	for i, r := range refs {
		if r == ref {
			return i
		}
	}
	panic("ref not found")
}
