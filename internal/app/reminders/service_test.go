package reminders

import "testing"

func TestParseBeforeMinutes(t *testing.T) {
	tests := map[string]int{
		"30":    30,
		"30m":   30,
		"1h":    60,
		"2h15m": 135,
	}
	for input, want := range tests {
		got, err := ParseBeforeMinutes(input)
		if err != nil {
			t.Fatalf("ParseBeforeMinutes(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseBeforeMinutes(%q)=%d want=%d", input, got, want)
		}
	}
}

func TestParseAtDate(t *testing.T) {
	got, err := ParseAtDate("2026-02-23 10:00")
	if err != nil {
		t.Fatalf("ParseAtDate: %v", err)
	}
	if got != "2026-02-23T10:00:00" {
		t.Fatalf("unexpected parsed date: %q", got)
	}
}

func TestValidateTimeChoice(t *testing.T) {
	if err := ValidateTimeChoice("", ""); err == nil {
		t.Fatalf("expected error")
	}
	if err := ValidateTimeChoice("10m", "2026-02-23"); err == nil {
		t.Fatalf("expected conflict error")
	}
	if err := ValidateTimeChoice("10m", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
