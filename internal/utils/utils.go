package utils

import "slices"

func InsertSorted(arr []int, v int) []int {
	pos, _ := slices.BinarySearch(arr, v)
	arr = slices.Insert(arr, pos, v)
	return arr
}

func InsertSortedFunc[S ~[]E, E any](arr S, v E, cmp func(E, E) int) S {
	pos, _ := slices.BinarySearchFunc(arr, v, cmp)
	arr = slices.Insert(arr, pos, v)
	return arr
}
