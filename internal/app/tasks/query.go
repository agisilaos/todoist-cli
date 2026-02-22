package tasks

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var relativeAgoPattern = regexp.MustCompile(`^([0-9]+)\s+(day|days|week|weeks)\s+ago$`)

func IsLikelyLiteralFilter(filter string) bool {
	value := strings.TrimSpace(filter)
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "@#|&!:()[]{}") {
		return false
	}
	return true
}

func ToSearchFilter(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	escaped := replacer.Replace(strings.TrimSpace(value))
	return fmt.Sprintf(`search: "%s"`, escaped)
}

func NormalizeCompletedDateRange(now time.Time, since, until string) (string, string, error) {
	var err error
	since = strings.TrimSpace(since)
	until = strings.TrimSpace(until)
	if since != "" {
		since, err = NormalizeCompletedDateValue(since, now)
		if err != nil {
			return "", "", err
		}
		if until == "" {
			until = now.UTC().Format("2006-01-02")
		}
	}
	if until != "" {
		until, err = NormalizeCompletedDateValue(until, now)
		if err != nil {
			return "", "", err
		}
	}
	if since != "" && until != "" && since > until {
		return "", "", fmt.Errorf("--since must be on or before --until")
	}
	return since, until, nil
}

func NormalizeCompletedDateValue(value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Format("2006-01-02"), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC().Format("2006-01-02"), nil
	}
	lower := strings.ToLower(value)
	switch lower {
	case "today":
		return now.UTC().Format("2006-01-02"), nil
	case "yesterday":
		return now.UTC().AddDate(0, 0, -1).Format("2006-01-02"), nil
	case "tomorrow":
		return now.UTC().AddDate(0, 0, 1).Format("2006-01-02"), nil
	}
	if weekday, ok := parseWeekday(lower); ok {
		return mostRecentWeekday(now.UTC(), weekday).Format("2006-01-02"), nil
	}
	if m := relativeAgoPattern.FindStringSubmatch(lower); len(m) == 3 {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "day", "days":
			return now.UTC().AddDate(0, 0, -n).Format("2006-01-02"), nil
		case "week", "weeks":
			return now.UTC().AddDate(0, 0, -(n * 7)).Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid date %q; use YYYY-MM-DD, RFC3339, today/yesterday, weekday name, or '<N> days ago'", value)
}

func parseWeekday(value string) (time.Weekday, bool) {
	switch value {
	case "sunday":
		return time.Sunday, true
	case "monday":
		return time.Monday, true
	case "tuesday":
		return time.Tuesday, true
	case "wednesday":
		return time.Wednesday, true
	case "thursday":
		return time.Thursday, true
	case "friday":
		return time.Friday, true
	case "saturday":
		return time.Saturday, true
	default:
		return time.Sunday, false
	}
}

func mostRecentWeekday(now time.Time, weekday time.Weekday) time.Time {
	diff := (int(now.Weekday()) - int(weekday) + 7) % 7
	return now.AddDate(0, 0, -diff)
}
