package settings

import "testing"

func TestParseUpdateInput(t *testing.T) {
	input, err := ParseUpdateInput(UpdateFlags{
		Timezone:     "UTC",
		TimeFormat:   "24",
		DateFormat:   "intl",
		StartDay:     "monday",
		Theme:        "kale",
		AutoReminder: "30",
		NextWeek:     "friday",
		ReminderPush: "on",
	})
	if err != nil {
		t.Fatalf("ParseUpdateInput: %v", err)
	}
	if input.Timezone == nil || *input.Timezone != "UTC" {
		t.Fatalf("expected timezone")
	}
	if input.Theme == nil || *input.Theme != 5 {
		t.Fatalf("expected parsed theme")
	}
	if input.ReminderPush == nil || !*input.ReminderPush {
		t.Fatalf("expected reminder push true")
	}
}

func TestParseUpdateInputRejectsInvalidTheme(t *testing.T) {
	if _, err := ParseUpdateInput(UpdateFlags{Theme: "invalid"}); err == nil {
		t.Fatalf("expected invalid theme error")
	}
}

func TestHasUpdate(t *testing.T) {
	input, err := ParseUpdateInput(UpdateFlags{Timezone: "UTC"})
	if err != nil {
		t.Fatalf("ParseUpdateInput: %v", err)
	}
	if !HasUpdate(input) {
		t.Fatalf("expected update detection")
	}
}
