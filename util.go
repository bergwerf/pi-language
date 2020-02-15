package main

import (
	"container/list"
	"fmt"
	"path/filepath"
)

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

// Create a deep copy of a string to uint map.
func copyMap(m map[string]uint) map[string]uint {
	n := make(map[string]uint, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
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

// Loc contains a file location.
type Loc struct {
	Path    string
	Ln, Col int
}

func (l Loc) String() string {
	if len(l.Path) == 0 {
		return "<internal>"
	}
	return fmt.Sprintf("%v:%v:%v", filepath.Base(l.Path), l.Ln, l.Col)
}

// Set is a hash set using a map.
type Set map[string]bool

// MakeSet makes a new Set.
func MakeSet() Set {
	return Set(map[string]bool{})
}

// Add element.
func (s Set) Add(k string) {
	s[k] = true
}

// Remove element.
func (s Set) Remove(k string) {
	delete(s, k)
}

// AddAll elements.
func (s Set) AddAll(a []string) {
	for _, k := range a {
		s.Add(k)
	}
}

// Contains element.
func (s Set) Contains(k string) bool {
	if _, in := s[k]; in {
		return true
	}
	return false
}

// ToSlice returns a slice of elements.
func (s Set) ToSlice() []string {
	elements := make([]string, 0, len(s))
	for k := range s {
		elements = append(elements, k)
	}
	return elements
}
