package dsa

import (
	"iter"
)

// A Set is an unordered collection of unique members
type Set[T comparable] map[T]struct{}

func NewSet[T comparable](items ...T) Set[T] {
	s := make(Set[T], len(items))
	for _, v := range items {
		s.Add(v)
	}
	return s
}

func (s Set[T]) Add(v T) {
	s[v] = struct{}{}
}

func (s Set[T]) AddFromSlice(slc []T) int {
	added := 0
	for _, v := range slc {
		if !s.Contains(v) {
			s.Add(v)
			added += 1
		}
	}
	return added
}

func (s Set[T]) Remove(item T) {
	delete(s, item)
}

func (s Set[T]) Contains(item T) bool {
	_, exists := s[item]
	return exists
}

func (s Set[T]) Size() int {
	return len(s)
}

func (s Set[T]) ToSlice() []T {
	slice := make([]T, 0, s.Size())
	for item := range s {
		slice = append(slice, item)
	}
	return slice
}

// --- Set Operations ---

func (s Set[T]) Union(other Set[T]) Set[T] {
	res := make(Set[T], s.Size()+other.Size())
	for item := range s {
		res.Add(item)
	}
	for item := range other {
		res.Add(item)
	}
	return res
}

// UnionN returns a new set containing all elements from s and all provided sets.
// If no additional sets are passed, it returns a shallow copy of s.
func (s Set[T]) UnionN(others ...Set[T]) Set[T] {
	totalCap := s.Size()
	for _, other := range others {
		totalCap += other.Size()
	}

	res := make(Set[T], totalCap)
	for item := range s {
		res.Add(item)
	}
	for _, other := range others {
		for item := range other {
			res.Add(item)
		}
	}
	return res
}

func (s Set[T]) Intersection(other Set[T]) Set[T] {
	small, large := s, other
	if s.Size() > other.Size() {
		small, large = other, s
	}

	res := make(Set[T])
	for item := range small {
		if large.Contains(item) {
			res.Add(item)
		}
	}
	return res
}

func (s Set[T]) IntersectN(others ...Set[T]) Set[T] {
	if s.Size() == 0 {
		return NewSet[T]()
	}

	smallest := s
	for _, other := range others {
		if other.Size() == 0 {
			// intersection with an empty set is always empty
			return NewSet[T]()
		}
		if other.Size() < smallest.Size() {
			smallest = other
		}
	}

	res := make(Set[T], len(smallest))
	for item := range smallest {
		// Check s if s wasn't the smallest set
		if !s.Contains(item) {
			continue
		}

		inAll := true
		for _, other := range others {
			if !other.Contains(item) {
				inAll = false
				break
			}
		}
		if inAll {
			res.Add(item)
		}
	}
	return res
}

func (s Set[T]) LimitIntersectN(limit int64, others ...Set[T]) int64 {
	if s.Size() == 0 {
		return 0
	}

	if limit < 0 {
		limit = 0
	}

	smallest := s
	for _, other := range others {
		if other.Size() == 0 {
			// intersection with an empty set is always empty
			return 0
		}
		if other.Size() < smallest.Size() {
			smallest = other
		}
	}

	// res := make(Set[T], len(smallest))
	var card int64
	for item := range smallest {
		// Check s if s wasn't the smallest set
		if !s.Contains(item) {
			continue
		}

		inAll := true
		for _, other := range others {
			if !other.Contains(item) {
				inAll = false
				break
			}
		}
		if inAll {
			// res.Add(item)
			card += 1
		}

		if limit > 0 && card == limit {
			return limit
		}
	}
	return card
}

func (s Set[T]) Difference(other Set[T]) Set[T] {
	res := make(Set[T])
	for item := range s {
		if !other.Contains(item) {
			res.Add(item)
		}
	}
	return res
}

func (s Set[T]) SymmetricDifference(other Set[T]) Set[T] {
	res := make(Set[T])
	for item := range s {
		if !other.Contains(item) {
			res.Add(item)
		}
	}
	for item := range other {
		if !s.Contains(item) {
			res.Add(item)
		}
	}
	return res
}

// DifferenceN returns a new set containing elements in s that are not in any of the provided sets.
// If no additional sets are passed, it returns a shallow copy of s.
func (s Set[T]) DifferenceN(others ...Set[T]) Set[T] {
	if len(s) == 0 {
		return NewSet[T]()
	}

	res := make(Set[T], len(s))
	for item := range s {
		inAny := false
		for _, other := range others {
			if other.Contains(item) {
				inAny = true
				break
			}
		}
		if !inAny {
			res.Add(item)
		}
	}
	return res
}

// --- Comparison Checks ---

// IsSubset reports whether all elements of s are also in other (s ⊆ other).
func (s Set[T]) IsSubset(other Set[T]) bool {
	if s.Size() > other.Size() {
		return false
	}
	for item := range s {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// IsSuperset reports whether s contains all elements of other (s ⊇ other).
func (s Set[T]) IsSuperset(other Set[T]) bool {
	return other.IsSubset(s)
}

// IsDisjoint reports whether s and other have no elements in common (s ∩ other = ∅).
func (s Set[T]) IsDisjoint(other Set[T]) bool {
	small, large := s, other
	if s.Size() > other.Size() {
		small, large = other, s
	}
	for item := range small {
		if large.Contains(item) {
			return false
		}
	}
	return true
}

// Equals reports whether s and other contain the exact same elements (s == other).
func (s Set[T]) Equals(other Set[T]) bool {
	if s.Size() != other.Size() {
		return false
	}
	for item := range s {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// Items returns a generator function for iterating each element in s by using
// the range keyword.
//
//	for element := range s.Items() { ... }
func (s Set[T]) Items() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := range s {
			if !yield(item) {
				return
			}
		}
	}
}
