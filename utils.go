package main

import "iter"

func Map[T, K any](seq iter.Seq[T], fn func(v T) K) iter.Seq[K] {
	return func(yield func(K) bool) {
		for v := range seq {
			if !yield(fn(v)) {
				return
			}
		}
	}
}
