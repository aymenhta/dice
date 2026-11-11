package dsa

import (
	"reflect"
	"testing"
)

func TestNewDeque(t *testing.T) {
	d := NewDeque[int]()
	if !d.IsEmpty() {
		t.Errorf("expected new deque to be empty")
	}
	if d.Len() != 0 {
		t.Errorf("expected length 0, got %d", d.Len())
	}

	if _, ok := d.Front(); ok {
		t.Errorf("expected Front() to return false on empty deque")
	}
	if _, ok := d.Back(); ok {
		t.Errorf("expected Back() to return false on empty deque")
	}
	if _, ok := d.PopFront(); ok {
		t.Errorf("expected PopFront() to return false on empty deque")
	}
	if _, ok := d.PopBack(); ok {
		t.Errorf("expected PopBack() to return false on empty deque")
	}
}

func TestPushAndPop(t *testing.T) {
	d := NewDeque[string]()

	// Push items to both ends
	d.PushBack("b")
	d.PushBack("c")
	d.PushFront("a")

	// Order should be: "a", "b", "c"
	if d.Len() != 3 {
		t.Fatalf("expected len 3, got %d", d.Len())
	}

	if val, ok := d.Front(); !ok || val != "a" {
		t.Errorf("expected Front() to be 'a', got %v (ok=%v)", val, ok)
	}
	if val, ok := d.Back(); !ok || val != "c" {
		t.Errorf("expected Back() to be 'c', got %v (ok=%v)", val, ok)
	}

	// Pop front -> "a"
	if val, ok := d.PopFront(); !ok || val != "a" {
		t.Errorf("expected PopFront() to be 'a', got %v (ok=%v)", val, ok)
	}

	// Pop back -> "c"
	if val, ok := d.PopBack(); !ok || val != "c" {
		t.Errorf("expected PopBack() to be 'c', got %v (ok=%v)", val, ok)
	}

	// Remaining -> "b"
	if d.Len() != 1 {
		t.Fatalf("expected len 1, got %d", d.Len())
	}

	if val, ok := d.PopFront(); !ok || val != "b" {
		t.Errorf("expected PopFront() to be 'b', got %v (ok=%v)", val, ok)
	}

	if !d.IsEmpty() {
		t.Errorf("expected deque to be empty now")
	}
}

func TestToSlice(t *testing.T) {
	d := NewDeque[int]()
	d.PushBack(2)
	d.PushFront(1)
	d.PushBack(3)

	got := d.ToSlice()
	want := []int{1, 2, 3}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToSlice() = %v, want %v", got, want)
	}
}

func TestSlice(t *testing.T) {
	d := NewDeque[int]()
	for i := 1; i <= 5; i++ {
		d.PushBack(i)
	}
	// Deque elements: [1, 2, 3, 4, 5]

	tests := []struct {
		name      string
		start     int
		end       int
		want      []int
		wantPanic bool
	}{
		{"Full slice", 0, 5, []int{1, 2, 3, 4, 5}, false},
		{"Sub-slice middle", 1, 4, []int{2, 3, 4}, false},
		{"Sub-slice start", 0, 2, []int{1, 2}, false},
		{"Sub-slice end", 3, 5, []int{4, 5}, false},
		{"Empty slice range", 2, 2, []int{}, false},
		{"Invalid start < 0", -1, 3, nil, true},
		{"Invalid end > len", 0, 6, nil, true},
		{"Invalid start > end", 3, 2, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("Slice(%d, %d) panic = %v, wantPanic %v", tt.start, tt.end, r != nil, tt.wantPanic)
				}
			}()

			got := d.Slice(tt.start, tt.end)
			if !tt.wantPanic && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Slice(%d, %d) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}
}
