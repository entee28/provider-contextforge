// Package sliceutil provides small comparison helpers shared across the
// gateway, virtualserver, and a2aagent controllers' isUpToDate checks.
package sliceutil

import (
	"cmp"
	"slices"
)

// EqualUnordered reports whether a and b contain the same elements,
// ignoring order, without mutating the inputs.
func EqualUnordered[T cmp.Ordered](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}

	a2, b2 := append([]T(nil), a...), append([]T(nil), b...)
	slices.Sort(a2)
	slices.Sort(b2)
	return slices.Equal(a2, b2)
}
