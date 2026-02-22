package tasks

import (
	"testing"
	"time"
)

func TestIsLikelyLiteralFilter(t *testing.T) {
	if !IsLikelyLiteralFilter("Call mom") {
		t.Fatalf("expected literal text filter to be true")
	}
	if IsLikelyLiteralFilter("@work & today") {
		t.Fatalf("expected structured query to be false")
	}
}

func TestToSearchFilterEscapes(t *testing.T) {
	got := ToSearchFilter(`say "hi"`)
	want := `search: "say \"hi\""`
	if got != want {
		t.Fatalf("unexpected search filter: got %q want %q", got, want)
	}
}

func TestNormalizeCompletedDateRange(t *testing.T) {
	now := time.Date(2026, 2, 22, 18, 0, 0, 0, time.UTC)
	since, until, err := NormalizeCompletedDateRange(now, "30 days ago", "")
	if err != nil {
		t.Fatalf("NormalizeCompletedDateRange: %v", err)
	}
	if since != "2026-01-23" || until != "2026-02-22" {
		t.Fatalf("unexpected range: since=%q until=%q", since, until)
	}
}

func TestNormalizeCompletedDateValueWeekday(t *testing.T) {
	now := time.Date(2026, 2, 22, 18, 0, 0, 0, time.UTC) // Sunday
	got, err := NormalizeCompletedDateValue("monday", now)
	if err != nil {
		t.Fatalf("NormalizeCompletedDateValue: %v", err)
	}
	if got != "2026-02-16" {
		t.Fatalf("unexpected monday date: %q", got)
	}
}
