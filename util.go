package main

import (
	"fmt"
	"path/filepath"
)

func copyStrIntMap(m map[string]int) map[string]int {
	n := make(map[string]int, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}

func castStrSliceToInterface(src []string) []interface{} {
	dst := make([]interface{}, len(src))
	for i, str := range src {
		dst[i] = str
	}
	return dst
}

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
type Set map[interface{}]bool

// MakeSet makes a new Set.
func MakeSet() Set {
	return Set(map[interface{}]bool{})
}

// Add element.
func (s Set) Add(k interface{}) {
	s[k] = true
}

// Remove element.
func (s Set) Remove(k interface{}) {
	delete(s, k)
}

// AddAll elements.
func (s Set) AddAll(a ...interface{}) {
	for _, k := range a {
		s.Add(k)
	}
}

// Union with another set.
func (s Set) Union(t Set) {
	for k := range t {
		s.Add(k)
	}
}

// Contains element.
func (s Set) Contains(k interface{}) bool {
	if _, in := s[k]; in {
		return true
	}
	return false
}

// Copy returns a deep copy of the set.
func (s Set) Copy() Set {
	newSet := Set(make(map[interface{}]bool, len(s)))
	for k := range s {
		newSet.Add(k)
	}
	return newSet
}
