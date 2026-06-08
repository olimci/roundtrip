package list

import (
	"slices"
	"testing"
)

func TestValues(t *testing.T) {
	var l List[string]
	l.PushBack("a")
	mid := l.PushBack("mid")
	l.PushBack("b")
	l.InsertBefore(mid, "before")
	l.InsertAfter(mid, "after")

	got := slices.Collect(l.Values())
	want := []string{"a", "before", "mid", "after", "b"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if l.Len() != 5 {
		t.Fatalf("len got %d, want 5", l.Len())
	}
}

func TestValuesRange(t *testing.T) {
	l := FromSlice([]string{"a", "b", "c", "d"})

	got := slices.Collect(ValuesRange(l.Head.Next, l.Tail.Prev))
	want := []string{"b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestElems(t *testing.T) {
	var l List[int]
	first := l.PushBack(1)
	second := l.PushBack(2)
	third := l.PushBack(3)

	var got []*Elem[int]
	for e := range l.Elems() {
		got = append(got, e)
	}
	if !slices.Equal(got, []*Elem[int]{first, second, third}) {
		t.Fatal("Elems did not yield list elements in order")
	}
}

func TestRemoveRelinksNeighbors(t *testing.T) {
	var l List[int]
	first := l.PushBack(1)
	mid := l.PushBack(2)
	last := l.PushBack(3)

	if got := l.Remove(mid); got != 2 {
		t.Fatalf("removed got %d, want 2", got)
	}
	if first.Next != last || last.Prev != first {
		t.Fatal("remove did not relink neighboring elements")
	}

	got := slices.Collect(l.Values())
	want := []int{1, 3}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestSet(t *testing.T) {
	var l List[string]
	e := l.PushBack("old")
	e.Value = "new"

	got := slices.Collect(l.Values())
	want := []string{"new"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestInsertListTransfersOwnership(t *testing.T) {
	var dst List[string]
	a := dst.PushBack("a")
	dst.PushBack("d")
	src := FromSlice([]string{"b", "c"})

	dst.InsertListAfter(a, src)

	got := slices.Collect(dst.Values())
	want := []string{"a", "b", "c", "d"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if src.Len() != 0 || src.Head != nil || src.Tail != nil {
		t.Fatal("source list was not emptied after insertion")
	}
}

func TestReplaceRange(t *testing.T) {
	l := FromSlice([]int{1, 2, 3, 4, 5})
	first := l.Head.Next
	last := first.Next.Next

	l.ReplaceRange(first, last, FromSlice([]int{20, 30}))

	got := slices.Collect(l.Values())
	want := []int{1, 20, 30, 5}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestCutRangeTransfersOwnership(t *testing.T) {
	l := FromSlice([]int{1, 2, 3, 4, 5})
	first := l.Head.Next
	last := first.Next.Next

	cut := l.CutRange(first, last)

	got := slices.Collect(l.Values())
	want := []int{1, 5}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	gotCut := slices.Collect(cut.Values())
	wantCut := []int{2, 3, 4}
	if !slices.Equal(gotCut, wantCut) {
		t.Fatalf("cut got %#v, want %#v", gotCut, wantCut)
	}
	if first.Prev != nil || last.Next != nil {
		t.Fatal("cut range is still linked to original list")
	}
}

func TestCloneCopiesElements(t *testing.T) {
	l := FromSlice([]string{"a", "b", "c"})

	clone, elems := l.Clone()

	got := slices.Collect(clone.Values())
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("clone got %#v, want %#v", got, want)
	}
	if elems[l.Head] != clone.Head || elems[l.Tail] != clone.Tail {
		t.Fatal("element map does not point to cloned elements")
	}
	if elems[l.Head] == l.Head || elems[l.Tail] == l.Tail {
		t.Fatal("clone reused original elements")
	}

	clone.Remove(clone.Head)
	got = slices.Collect(l.Values())
	if !slices.Equal(got, want) {
		t.Fatalf("mutating clone changed original to %#v", got)
	}
}

func TestCloneRangeCopiesElements(t *testing.T) {
	l := FromSlice([]string{"a", "b", "c", "d"})

	clone, elems := CloneRange(l.Head.Next, l.Tail.Prev)

	got := slices.Collect(clone.Values())
	want := []string{"b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("clone range got %#v, want %#v", got, want)
	}
	if elems[l.Head.Next] != clone.Head || elems[l.Tail.Prev] != clone.Tail {
		t.Fatal("element map does not point to cloned range elements")
	}
	if _, ok := elems[l.Head]; ok {
		t.Fatal("element map contains element before cloned range")
	}
}

func TestReplaceWithEmptyListRemovesRange(t *testing.T) {
	l := FromSlice([]int{1, 2, 3, 4})

	l.ReplaceRange(l.Head.Next, l.Tail, new(List[int]))

	got := slices.Collect(l.Values())
	want := []int{1}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestSliceHelpers(t *testing.T) {
	l := FromSlice([]string{"b"})
	l.PushFrontList(FromSlice([]string{"a"}))
	l.PushBackList(FromSlice([]string{"d"}))
	l.InsertListAfter(l.Head.Next, FromSlice([]string{"c"}))

	got := slices.Collect(l.Values())
	want := []string{"a", "b", "c", "d"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
