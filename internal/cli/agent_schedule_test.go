package cli

import "testing"

func TestParseWeeklySpec(t *testing.T) {
	spec, err := parseWeeklySpec("sat 09:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Weekday != 7 || spec.Hour != 9 || spec.Minute != 30 {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestCronLine(t *testing.T) {
	spec := scheduleSpec{Weekday: 7, Hour: 9, Minute: 0}
	line := cronLine(spec, "/usr/local/bin/todoist", []string{"agent", "run", "--instruction", "hello"})
	want := "0 9 * * 6 /usr/local/bin/todoist agent run --instruction hello"
	if line != want {
		t.Fatalf("unexpected cron line: %q", line)
	}
}
