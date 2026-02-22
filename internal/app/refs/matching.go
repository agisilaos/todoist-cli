package refs

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Candidate struct {
	ID   string
	Name string
	Rank int
}

type AmbiguousMatchError struct {
	Entity  string
	Input   string
	Matches []string
}

func (e *AmbiguousMatchError) Error() string {
	return fmt.Sprintf("ambiguous %s match for %q; matches: %s", e.Entity, e.Input, strings.Join(e.Matches, ", "))
}

func StripIDPrefix(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "id:") {
		return strings.TrimSpace(value[3:])
	}
	return value
}

func IsNumeric(value string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil
}

func NormalizeRef(value string) (normalized string, directID bool) {
	original := strings.TrimSpace(value)
	if original == "" {
		return "", false
	}
	explicitID := strings.HasPrefix(strings.ToLower(original), "id:")
	normalized = StripIDPrefix(original)
	if explicitID || IsNumeric(normalized) {
		return normalized, true
	}
	return normalized, false
}

func FuzzyCandidates[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) []Candidate {
	var out []Candidate
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return nil
	}
	for _, item := range items {
		name := strings.TrimSpace(nameFn(item))
		rank, ok := candidateRank(lower, strings.ToLower(name))
		if ok {
			out = append(out, Candidate{ID: idFn(item), Name: name, Rank: rank})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func ResolveFuzzy[T any](value string, items []T, nameFn func(T) string, idFn func(T) string) (string, []Candidate) {
	candidates := FuzzyCandidates(value, items, nameFn, idFn)
	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}
	if len(candidates) > 1 {
		return "", candidates
	}
	return "", nil
}

func candidateRank(query, target string) (int, bool) {
	if query == "" || target == "" {
		return 0, false
	}
	if target == query {
		return 0, true
	}
	if strings.HasPrefix(target, query) {
		return 100 + len(target) - len(query), true
	}
	if idx := strings.Index(target, query); idx >= 0 {
		return 200 + (idx * 8) + (len(target) - len(query)), true
	}
	gap, ok := subsequenceGap(query, target)
	if ok {
		return 400 + gap + (len(target) - len(query)), true
	}
	return 0, false
}

func subsequenceGap(query, target string) (int, bool) {
	qi := 0
	prev := -1
	gap := 0
	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] != query[qi] {
			continue
		}
		if prev >= 0 {
			gap += ti - prev - 1
		}
		prev = ti
		qi++
	}
	return gap, qi == len(query)
}
