package dsa

// node represents an element in the doubly linked list.
type node[T any] struct {
	prev  *node[T]
	next  *node[T]
	value T
}

// Deque is a generic double-ended queue implemented using a doubly linked list.
type Deque[T any] struct {
	head *node[T]
	tail *node[T]
	size int
}

// NewDeque creates and returns an empty Deque.
func NewDeque[T any]() *Deque[T] {
	return &Deque[T]{}
}

// Len returns the number of elements in the deque.
func (d *Deque[T]) Len() int {
	return d.size
}

// IsEmpty returns true if the deque contains no elements.
func (d *Deque[T]) IsEmpty() bool {
	return d.size == 0
}

// PushFront adds an element to the front of the deque.
func (d *Deque[T]) PushFront(val T) {
	newNode := &node[T]{value: val}
	if d.size == 0 {
		d.head = newNode
		d.tail = newNode
	} else {
		newNode.next = d.head
		d.head.prev = newNode
		d.head = newNode
	}
	d.size++
}

// PushBack adds an element to the back of the deque.
func (d *Deque[T]) PushBack(val T) {
	newNode := &node[T]{value: val}
	if d.size == 0 {
		d.head = newNode
		d.tail = newNode
	} else {
		newNode.prev = d.tail
		d.tail.next = newNode
		d.tail = newNode
	}
	d.size++
}

// PopFront removes and returns the element from the front of the deque.
func (d *Deque[T]) PopFront() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	val := d.head.value
	d.head = d.head.next
	if d.head != nil {
		d.head.prev = nil
	} else {
		d.tail = nil
	}
	d.size--
	return val, true
}

// PopBack removes and returns the element from the back of the deque.
func (d *Deque[T]) PopBack() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	val := d.tail.value
	d.tail = d.tail.prev
	if d.tail != nil {
		d.tail.next = nil
	} else {
		d.head = nil
	}
	d.size--
	return val, true
}

// Front returns the element at the front without removing it.
func (d *Deque[T]) Front() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	return d.head.value, true
}

// Back returns the element at the back without removing it.
func (d *Deque[T]) Back() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	return d.tail.value, true
}

// ToSlice converts the entire deque into a standard Go slice from front to back.
// Time Complexity: O(N)
func (d *Deque[T]) ToSlice() []T {
	result := make([]T, 0, d.size)
	curr := d.head
	for curr != nil {
		result = append(result, curr.value)
		curr = curr.next
	}
	return result
}

// At returns the value at the specified zero-based index.
// Returns the value and true if successful, or the zero value and false if out of bounds.
// Time Complexity: O(N/2) - optimized to traverse from head or tail depending on index position.
func (d *Deque[T]) At(index int) (T, bool) {
	if index < 0 || index >= d.size {
		var zero T
		return zero, false
	}

	var curr *node[T]
	// Traverse from the head if closer, or from the tail if closer
	if index < d.size/2 {
		curr = d.head
		for i := 0; i < index; i++ {
			curr = curr.next
		}
	} else {
		curr = d.tail
		for i := d.size - 1; i > index; i-- {
			curr = curr.prev
		}
	}
	return curr.value, true
}

// RemoveOcc removes occurrences of elements equal to 'element' based on 'count':
//   - count > 0: Remove elements moving from head to tail.
//   - count < 0: Remove elements moving from tail to head.
//   - count = 0: Remove all elements equal to element.
//
// Returns the number of elements removed.
// Time Complexity: O(N)
func (d *Deque[T]) RemoveOcc(count int, matchFn func(T) bool) int {
	if d.size == 0 {
		return 0
	}

	removed := 0

	if count > 0 {
		curr := d.head
		for curr != nil && removed < count {
			next := curr.next
			if matchFn(curr.value) {
				d.removeNode(curr)
				removed++
			}
			curr = next
		}
	} else if count < 0 {
		target := -count
		curr := d.tail
		for curr != nil && removed < target {
			prev := curr.prev
			if matchFn(curr.value) {
				d.removeNode(curr)
				removed++
			}
			curr = prev
		}
	} else {
		curr := d.head
		for curr != nil {
			next := curr.next
			if matchFn(curr.value) {
				d.removeNode(curr)
				removed++
			}
			curr = next
		}
	}

	return removed
}

func (d *Deque[T]) removeNode(curr *node[T]) {
	if curr.prev != nil {
		curr.prev.next = curr.next
	} else {
		d.head = curr.next
	}

	if curr.next != nil {
		curr.next.prev = curr.prev
	} else {
		d.tail = curr.prev
	}
	d.size--
}

// Slice extracts a sub-range [start, end) from the deque as a standard Go slice.
// Panics if indices are out of bounds, matching native Go slice behavior.
// Time Complexity: O(N)
func (d *Deque[T]) Slice(start, end int) []T {
	if start < 0 || end > d.size || start > end {
		panic("deque: slice bounds out of range")
	}

	result := make([]T, 0, end-start)
	curr := d.head

	// Fast-forward to the start index
	for i := 0; i < start; i++ {
		curr = curr.next
	}

	// Collect elements up to the end index
	for i := start; i < end; i++ {
		result = append(result, curr.value)
		curr = curr.next
	}

	return result
}
