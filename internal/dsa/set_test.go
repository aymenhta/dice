package dsa

import (
	"slices"
	"testing"
)

func TestSetBasics(t *testing.T) {
	s := NewSet("go", "rust", "go") // duplicate ignored on init

	if got := s.Size(); got != 2 {
		t.Fatalf("Size() = %d, want 2", got)
	}
	if !s.Contains("go") || !s.Contains("rust") {
		t.Errorf("expected set to contain 'go' and 'rust'")
	}
	if s.Contains("python") {
		t.Errorf("expected set not to contain 'python'")
	}

	s.Add("python")
	if !s.Contains("python") || s.Size() != 3 {
		t.Errorf("Add('python') failed, Size = %d", s.Size())
	}

	s.Remove("rust")
	if s.Contains("rust") || s.Size() != 2 {
		t.Errorf("Remove('rust') failed, Size = %d", s.Size())
	}

	// Removing a non-existent item should be a safe no-op
	s.Remove("non-existent")
	if s.Size() != 2 {
		t.Errorf("Remove('non-existent') changed Size to %d", s.Size())
	}
}

func TestSetToSlice(t *testing.T) {
	s := NewSet("b", "a", "c")
	got := s.ToSlice()
	slices.Sort(got) // Sort because map iteration order is randomized

	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("ToSlice() = %v, want %v", got, want)
	}
}

func TestSetOperations(t *testing.T) {
	a := NewSet("go", "rust", "python")
	b := NewSet("go", "typescript", "java")
	empty := NewSet[string]()

	tests := []struct {
		name string
		got  Set[string]
		want Set[string]
	}{
		{
			name: "Union",
			got:  a.Union(b),
			want: NewSet("go", "rust", "python", "typescript", "java"),
		},
		{
			name: "Union with empty",
			got:  a.Union(empty),
			want: a,
		},
		{
			name: "Intersection",
			got:  a.Intersection(b),
			want: NewSet("go"),
		},
		{
			name: "Intersection with empty",
			got:  a.Intersection(empty),
			want: empty,
		},
		{
			name: "Difference (A - B)",
			got:  a.Difference(b),
			want: NewSet("rust", "python"),
		},
		{
			name: "Difference (B - A)",
			got:  b.Difference(a),
			want: NewSet("typescript", "java"),
		},
		{
			name: "SymmetricDifference",
			got:  a.SymmetricDifference(b),
			want: NewSet("rust", "python", "typescript", "java"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.got.Equals(tt.want) {
				t.Errorf("%s = %v, want %v", tt.name, tt.got.ToSlice(), tt.want.ToSlice())
			}
		})
	}

	// Verify immutability: originals must remain unchanged
	if !a.Equals(NewSet("go", "rust", "python")) {
		t.Errorf("original set 'a' was mutated")
	}
}

func TestSetComparisons(t *testing.T) {
	sub := NewSet("go", "rust")
	super := NewSet("go", "rust", "python")
	same := NewSet("rust", "go")
	disjoint := NewSet("java", "c++")
	empty := NewSet[string]()

	// Equals
	if !sub.Equals(same) || !same.Equals(sub) {
		t.Error("expected sub and same to be equal")
	}
	if sub.Equals(super) {
		t.Error("expected sub and super not to be equal")
	}
	if !empty.Equals(NewSet[string]()) {
		t.Error("expected two empty sets to be equal")
	}

	// IsSubset & IsSuperset
	if !sub.IsSubset(super) {
		t.Error("expected sub to be a subset of super")
	}
	if super.IsSubset(sub) {
		t.Error("expected super not to be a subset of sub")
	}
	if !super.IsSuperset(sub) {
		t.Error("expected super to be a superset of sub")
	}
	if !empty.IsSubset(sub) {
		t.Error("expected empty set to be a subset of any set")
	}

	// IsDisjoint
	if !sub.IsDisjoint(disjoint) {
		t.Error("expected sub and disjoint to have no overlap")
	}
	if sub.IsDisjoint(super) {
		t.Error("expected overlapping sets not to be disjoint")
	}
	if !empty.IsDisjoint(sub) {
		t.Error("expected empty set to be disjoint from any set")
	}
}

func TestIntersectN(t *testing.T) {
	a := NewSet("go", "rust", "python", "java")
	b := NewSet("go", "rust", "typescript")
	c := NewSet("go", "c++", "rust")
	d := NewSet("swift", "kotlin")
	empty := NewSet[string]()

	tests := []struct {
		name   string
		base   Set[string]
		others []Set[string]
		want   Set[string]
	}{
		{
			name:   "Three overlapping sets",
			base:   a,
			others: []Set[string]{b, c},
			want:   NewSet("go", "rust"),
		},
		{
			name:   "No overlap across all sets",
			base:   a,
			others: []Set[string]{b, c, d},
			want:   empty,
		},
		{
			name:   "One of the sets is empty",
			base:   a,
			others: []Set[string]{b, empty, c},
			want:   empty,
		},
		{
			name:   "Base set is empty",
			base:   empty,
			others: []Set[string]{a, b},
			want:   empty,
		},
		{
			name:   "No other sets passed returns copy",
			base:   b,
			others: nil,
			want:   NewSet("go", "rust", "typescript"),
		},
		// --- Regression tests for equal-length sets with different elements ---
		{
			name:   "Equal length base and other with disjoint elements",
			base:   NewSet("go", "rust", "python"),
			others: []Set[string]{NewSet("java", "c++", "swift")},
			want:   empty,
		},
		{
			name:   "Equal length base and others with partial overlap",
			base:   NewSet("go", "rust", "python"),
			others: []Set[string]{NewSet("go", "java", "c++"), NewSet("go", "swift", "kotlin")},
			want:   NewSet("go"),
		},
		{
			name:   "Base is largest, two others tie for smallest with different elements",
			base:   NewSet("go", "rust", "python", "java"),
			others: []Set[string]{NewSet("go", "c++"), NewSet("rust", "swift")},
			want:   empty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.IntersectN(tt.others...)
			if !got.Equals(tt.want) {
				t.Errorf("IntersectN() = %v, want %v", got.ToSlice(), tt.want.ToSlice())
			}
		})
	}

	// Ensure passing no arguments returns an independent copy (mutation safety)
	copySet := b.IntersectN()
	copySet.Add("zig")
	if b.Contains("zig") {
		t.Error("mutating result of IntersectN() mutated original set")
	}
}

func TestUnionN(t *testing.T) {
	a := NewSet("go", "rust")
	b := NewSet("rust", "python")
	c := NewSet("java", "typescript", "go")
	empty := NewSet[string]()

	tests := []struct {
		name   string
		base   Set[string]
		others []Set[string]
		want   Set[string]
	}{
		{
			name:   "Multiple overlapping sets",
			base:   a,
			others: []Set[string]{b, c},
			want:   NewSet("go", "rust", "python", "java", "typescript"),
		},
		{
			name:   "Includes empty sets",
			base:   a,
			others: []Set[string]{empty, b, empty},
			want:   NewSet("go", "rust", "python"),
		},
		{
			name:   "Base set is empty",
			base:   empty,
			others: []Set[string]{a, b},
			want:   NewSet("go", "rust", "python"),
		},
		{
			name:   "All sets are empty",
			base:   empty,
			others: []Set[string]{empty, NewSet[string]()},
			want:   empty,
		},
		{
			name:   "No other sets passed returns copy",
			base:   a,
			others: nil,
			want:   NewSet("go", "rust"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.UnionN(tt.others...)
			if !got.Equals(tt.want) {
				t.Errorf("UnionN() = %v, want %v", got.ToSlice(), tt.want.ToSlice())
			}
		})
	}

	// Ensure original sets are not mutated and returned copy is independent
	copySet := a.UnionN()
	copySet.Add("zig")
	if a.Contains("zig") {
		t.Error("mutating result of UnionN() mutated original set")
	}
}
