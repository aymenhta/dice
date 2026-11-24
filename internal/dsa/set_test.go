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
