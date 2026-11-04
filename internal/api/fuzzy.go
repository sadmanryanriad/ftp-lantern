package api

import (
	"strings"
)

// Levenshtein calculates edit distance between two strings
func Levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost

			min := ins
			if del < min {
				min = del
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		copy(prev, curr)
	}

	return prev[lb]
}

// SlidingWindowMatch checks if query matches any substring window in target with distance <= maxDist
func SlidingWindowMatch(query, target string, maxDist int) bool {
	qLen := len(query)
	tLen := len(target)

	if qLen == 0 || tLen == 0 {
		return false
	}

	// Exact substring is 0 distance
	if strings.Contains(target, query) {
		return true
	}

	if tLen < qLen {
		return Levenshtein(query, target) <= maxDist
	}

	// Slide window across target
	for windowSize := qLen - 1; windowSize <= qLen+1; windowSize++ {
		if windowSize <= 0 || windowSize > tLen {
			continue
		}
		for i := 0; i <= tLen-windowSize; i++ {
			sub := target[i : i+windowSize]
			if Levenshtein(query, sub) <= maxDist {
				return true
			}
		}
	}

	return false
}
