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

func TestParseBeforeMinutesUsesTheCompleteDuration(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
	}{
		{"120s", 2}, {"59s", 1}, {"61s", 2}, {"30s30s", 1},
		{"1m30s", 2}, {"2 hours 15 minutes", 135},
		{"-30m", 0}, {"1.5h", 0}, {"junk30minutesJUNK", 0},
		{"1h trailing", 0}, {"0s", 0}, {"999999999999999999999999999", 0},
		{"999999999999999999999999999h", 0}, {"9223372036854775807h", 0},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseBeforeMinutes(tc.input)
			if tc.want == 0 {
				if err == nil {
					t.Fatalf("invalid duration accepted as %d minutes", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %d minutes, error %v; want %d minutes", got, err, tc.want)
			}
		})
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
