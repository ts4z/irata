// The ick package is for things I can't believe I have to write.
package ick

import (
	"math/rand"
)

// NShuffle shuffles a slice in-place.
//
// The "N" prefix is a nod to CL.
func NShuffle[T any](data []T) []T {
	rand.Shuffle(len(data), func(i, j int) { data[i], data[j] = data[j], data[i] })
	return data
}

// Shuffle copies an array, and then shuffles it.  This will work on an array
// of values, but you probably want to pass it an array of pointers.
func Shuffle[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	NShuffle(out)
	return out
}
