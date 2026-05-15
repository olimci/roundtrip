package sliceutil

func Map[I, O any](input []I, f func(I) O) []O {
	o := make([]O, len(input))
	for i, v := range input {
		o[i] = f(v)
	}
	return o
}
