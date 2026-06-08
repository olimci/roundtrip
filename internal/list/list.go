package list

import "iter"

// List is a mutable doubly linked list.
type List[T any] struct {
	Head *Elem[T]
	Tail *Elem[T]
	len  int
}

// Elem is one element owned by a List.
type Elem[T any] struct {
	Prev *Elem[T]
	Next *Elem[T]
	list *List[T]

	Value T
}

// FromSlice returns a new list containing values in order.
func FromSlice[T any](values []T) *List[T] {
	l := new(List[T])
	for _, v := range values {
		l.PushBack(v)
	}
	return l
}

// Len returns the number of elements in l.
func (l *List[T]) Len() int {
	return l.len
}

// PushFront inserts v at the front of l and returns the new element.
func (l *List[T]) PushFront(v T) *Elem[T] {
	return l.insert(nil, l.Head, v)
}

// PushBack inserts v at the back of l and returns the new element.
func (l *List[T]) PushBack(v T) *Elem[T] {
	return l.insert(l.Tail, nil, v)
}

// InsertBefore inserts v immediately before mark.
func (l *List[T]) InsertBefore(mark *Elem[T], v T) *Elem[T] {
	l.check(mark)
	return l.insert(mark.Prev, mark, v)
}

// InsertAfter inserts v immediately after mark.
func (l *List[T]) InsertAfter(mark *Elem[T], v T) *Elem[T] {
	l.check(mark)
	return l.insert(mark, mark.Next, v)
}

// PushFrontList moves every element from src to the front of l.
func (l *List[T]) PushFrontList(src *List[T]) {
	l.splice(nil, l.Head, src)
}

// PushBackList moves every element from src to the back of l.
func (l *List[T]) PushBackList(src *List[T]) {
	l.splice(l.Tail, nil, src)
}

// InsertListBefore moves every element from src immediately before mark.
func (l *List[T]) InsertListBefore(mark *Elem[T], src *List[T]) {
	l.check(mark)
	l.splice(mark.Prev, mark, src)
}

// InsertListAfter moves every element from src immediately after mark.
func (l *List[T]) InsertListAfter(mark *Elem[T], src *List[T]) {
	l.check(mark)
	l.splice(mark, mark.Next, src)
}

// Remove removes e from l and returns its value.
func (l *List[T]) Remove(e *Elem[T]) T {
	l.check(e)
	if e.Prev != nil {
		e.Prev.Next = e.Next
	} else {
		l.Head = e.Next
	}
	if e.Next != nil {
		e.Next.Prev = e.Prev
	} else {
		l.Tail = e.Prev
	}
	e.Prev = nil
	e.Next = nil
	e.list = nil
	l.len--
	return e.Value
}

// CutRange removes the inclusive range [first, last] from l and returns it as a
// new list.
func (l *List[T]) CutRange(first, last *Elem[T]) *List[T] {
	l.checkRange(first, last)
	prev, next := first.Prev, last.Next

	if prev != nil {
		prev.Next = next
	} else {
		l.Head = next
	}
	if next != nil {
		next.Prev = prev
	} else {
		l.Tail = prev
	}

	dst := &List[T]{Head: first, Tail: last}
	first.Prev = nil
	last.Next = nil
	for e := first; e != nil; e = e.Next {
		e.list = dst
		dst.len++
	}
	l.len -= dst.len
	return dst
}

// Clone returns a copy of l and a map from original elements to their copies.
func (l *List[T]) Clone() (*List[T], map[*Elem[T]]*Elem[T]) {
	dst := new(List[T])
	elems := map[*Elem[T]]*Elem[T]{}
	for e := range l.Elems() {
		elems[e] = dst.PushBack(e.Value)
	}
	return dst, elems
}

// CloneRange returns a copy of the inclusive range [first, last] and a map from
// original elements to their copies.
func CloneRange[T any](first, last *Elem[T]) (*List[T], map[*Elem[T]]*Elem[T]) {
	first.list.checkRange(first, last)
	dst := new(List[T])
	elems := map[*Elem[T]]*Elem[T]{}
	for e := first; ; e = e.Next {
		elems[e] = dst.PushBack(e.Value)
		if e == last {
			return dst, elems
		}
	}
}

// Replace replaces e with every element from src and returns e's value.
func (l *List[T]) Replace(e *Elem[T], src *List[T]) T {
	value := e.Value
	l.ReplaceRange(e, e, src)
	return value
}

// ReplaceRange replaces the inclusive range [first, last] with every element
// from src.
func (l *List[T]) ReplaceRange(first, last *Elem[T], src *List[T]) {
	l.checkRange(first, last)
	prev, next := first.Prev, last.Next
	removed := l.detachRange(first, next)
	l.len -= removed
	l.splice(prev, next, src)
}

// Elems iterates over l's elements in order.
func (l *List[T]) Elems() iter.Seq[*Elem[T]] {
	return func(yield func(*Elem[T]) bool) {
		for e := l.Head; e != nil; e = e.Next {
			if !yield(e) {
				return
			}
		}
	}
}

// Values iterates over l's element values in order.
func (l *List[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := l.Head; e != nil; e = e.Next {
			if !yield(e.Value) {
				return
			}
		}
	}
}

// ValuesRange iterates over the inclusive element range [first, last].
func ValuesRange[T any](first, last *Elem[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := first; e != nil; e = e.Next {
			if !yield(e.Value) || e == last {
				return
			}
		}
	}
}

func (l *List[T]) insert(prev, next *Elem[T], v T) *Elem[T] {
	e := &Elem[T]{Prev: prev, Next: next, list: l, Value: v}
	if prev != nil {
		prev.Next = e
	} else {
		l.Head = e
	}
	if next != nil {
		next.Prev = e
	} else {
		l.Tail = e
	}
	l.len++
	return e
}

func (l *List[T]) splice(prev, next *Elem[T], src *List[T]) {
	if src == l {
		panic("list: cannot splice list into itself")
	}
	if src.len == 0 {
		return
	}
	if prev != nil {
		prev.Next = src.Head
	} else {
		l.Head = src.Head
	}
	if next != nil {
		next.Prev = src.Tail
	} else {
		l.Tail = src.Tail
	}
	src.Head.Prev = prev
	src.Tail.Next = next
	for e := src.Head; e != next; e = e.Next {
		e.list = l
	}
	l.len += src.len
	src.Head = nil
	src.Tail = nil
	src.len = 0
}

func (l *List[T]) detachRange(first, after *Elem[T]) int {
	prev := first.Prev
	count := 0
	for e := first; e != after; {
		next := e.Next
		e.Prev = nil
		e.Next = nil
		e.list = nil
		count++
		e = next
	}
	if prev != nil {
		prev.Next = after
	} else {
		l.Head = after
	}
	if after != nil {
		after.Prev = prev
	} else {
		l.Tail = prev
	}
	return count
}

func (l *List[T]) check(e *Elem[T]) {
	if e == nil || e.list != l {
		panic("list: element does not belong to list")
	}
}

func (l *List[T]) checkRange(first, last *Elem[T]) {
	l.check(first)
	l.check(last)
	for e := first; e != nil; e = e.Next {
		if e == last {
			return
		}
	}
	panic("list: range end is before range start")
}
