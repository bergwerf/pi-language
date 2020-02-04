package main

import "container/list"

// Prepend the hd error to the tl errors if it is not nil.
func mergeErr(hd error, tl []error) []error {
	if hd != nil {
		return append([]error{hd}, tl...)
	}
	return tl
}

// Create a deep copy of a string to uint map.
func copyMap(m map[string]uint) map[string]uint {
	n := make(map[string]uint, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

// ListUnion merges the second list into the first list while retaining order.
func ListUnion(dst *list.List, src *list.List) {
	for e1, e2 := src.Front(), dst.Front(); e2 != nil; {
		if e1 == nil {
			dst.PushBack(e2.Value)
			e2.Next()
			continue
		}
		v1, v2 := e1.Value.(uint), e2.Value.(uint)
		if v1 == v2 {
			e2.Next()
		} else if v1 < v2 {
			e1.Next()
		} else {
			dst.InsertBefore(v2, e1)
		}
	}
}

// Convert list to uint slice.
func toSlice(l *list.List) []uint {
	output := make([]uint, 0, l.Len())
	for ptr := l.Front(); ptr != nil; ptr = ptr.Next() {
		output = append(output, ptr.Value.(uint))
	}
	return output
}

// Assertion shortcut
func assert(condition bool) {
	if !condition {
		panic("failed assertion")
	}
}
