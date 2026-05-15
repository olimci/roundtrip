package list

import "iter"

func New[T any]() *List[T] {
	return &List[T]{}
}

type List[T any] struct {
	Head *Elem[T]
	Tail *Elem[T]
	len  int
}

type Elem[T any] struct {
	Prev *Elem[T]
	Next *Elem[T]
	list *List[T]

	Value T
}

func FromSlice[T any](values []T) *List[T] {
	l := new(List[T])
	for _, v := range values {
		l.PushBack(v)
	}
	return l
}

func (l *List[T]) Len() int {
	return l.len
}

func (l *List[T]) PushFront(v T) *Elem[T] {
	return l.insert(nil, l.Head, v)
}

func (l *List[T]) PushBack(v T) *Elem[T] {
	return l.insert(l.Tail, nil, v)
}

func (l *List[T]) InsertBefore(mark *Elem[T], v T) *Elem[T] {
	l.check(mark)
	return l.insert(mark.Prev, mark, v)
}

func (l *List[T]) InsertAfter(mark *Elem[T], v T) *Elem[T] {
	l.check(mark)
	return l.insert(mark, mark.Next, v)
}

func (l *List[T]) PushFrontList(src *List[T]) {
	l.splice(nil, l.Head, src)
}

func (l *List[T]) PushBackList(src *List[T]) {
	l.splice(l.Tail, nil, src)
}

func (l *List[T]) InsertListBefore(mark *Elem[T], src *List[T]) {
	l.check(mark)
	l.splice(mark.Prev, mark, src)
}

func (l *List[T]) InsertListAfter(mark *Elem[T], src *List[T]) {
	l.check(mark)
	l.splice(mark, mark.Next, src)
}

func (l *List[T]) PushFrontSlice(values []T) {
	l.PushFrontList(FromSlice(values))
}

func (l *List[T]) PushBackSlice(values []T) {
	l.PushBackList(FromSlice(values))
}

func (l *List[T]) InsertSliceBefore(mark *Elem[T], values []T) {
	l.InsertListBefore(mark, FromSlice(values))
}

func (l *List[T]) InsertSliceAfter(mark *Elem[T], values []T) {
	l.InsertListAfter(mark, FromSlice(values))
}

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

func (l *List[T]) Replace(e *Elem[T], src *List[T]) T {
	value := e.Value
	l.ReplaceRange(e, e, src)
	return value
}

func (l *List[T]) ReplaceSlice(e *Elem[T], values []T) T {
	return l.Replace(e, FromSlice(values))
}

func (l *List[T]) ReplaceRange(first, last *Elem[T], src *List[T]) {
	l.checkRange(first, last)
	prev, next := first.Prev, last.Next
	removed := l.detachRange(first, next)
	l.len -= removed
	l.splice(prev, next, src)
}

func (l *List[T]) ReplaceRangeSlice(first, last *Elem[T], values []T) {
	l.ReplaceRange(first, last, FromSlice(values))
}

func (l *List[T]) Elems() iter.Seq[*Elem[T]] {
	return func(yield func(*Elem[T]) bool) {
		for e := l.Head; e != nil; e = e.Next {
			if !yield(e) {
				return
			}
		}
	}
}

func (l *List[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := l.Head; e != nil; e = e.Next {
			if !yield(e.Value) {
				return
			}
		}
	}
}

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
