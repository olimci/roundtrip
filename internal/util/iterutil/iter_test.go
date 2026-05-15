package iterutil

import (
	"slices"
	"testing"
)

func TestFilter(t *testing.T) {
	got := slices.Collect(Filter(slices.Values([]int{1, 2, 3, 4}), func(v int) bool {
		return v%2 == 0
	}))
	want := []int{2, 4}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestMap(t *testing.T) {
	got := slices.Collect(Map(slices.Values([]int{1, 2, 3}), func(v int) string {
		return string(rune('a' + v - 1))
	}))
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFilterMapPipeline(t *testing.T) {
	got := slices.Collect(Map(Filter(slices.Values([]int{1, 2, 3}), func(v int) bool {
		return v != 2
	}), func(v int) int {
		return v * 10
	}))
	want := []int{10, 30}
	if !slices.Equal(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
