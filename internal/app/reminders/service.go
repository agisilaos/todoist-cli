package reminders

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ParseBeforeMinutes(value string) (int, error) {
	raw := strings.TrimSpace(strings.ToLower(value))
	if raw == "" {
		return 0, errors.New("duration is required")
	}
	if allDigits(raw) {
		mins, _ := strconv.Atoi(raw)
		if mins <= 0 {
			return 0, errors.New("duration must be positive")
		}
		return mins, nil
	}
	re := regexp.MustCompile(`(\d+)\s*([hms]|hr|hrs|hour|hours|min|mins|minute|minutes)`)
	matches := re.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration: %s", value)
	}
	total := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "h", "hr", "hrs", "hour", "hours":
			total += n * 60
		case "m", "min", "mins", "minute", "minutes":
			total += n
		case "s":
			if n > 0 {
				total += 1
			}
		}
	}
	if total <= 0 {
		return 0, fmt.Errorf("invalid duration: %s", value)
	}
	return total, nil
}

func ParseAtDate(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", errors.New("at datetime is required")
	}
	if _, err := time.Parse(time.RFC3339, raw); err == nil {
		return raw, nil
	}
	if t, err := time.Parse("2006-01-02 15:04", raw); err == nil {
		return t.Format("2006-01-02T15:04:05"), nil
	}
	if _, err := time.Parse("2006-01-02", raw); err == nil {
		return raw, nil
	}
	return "", fmt.Errorf("invalid datetime: %s", value)
}

func ValidateTimeChoice(before, at string) error {
	hasBefore := strings.TrimSpace(before) != ""
	hasAt := strings.TrimSpace(at) != ""
	if !hasBefore && !hasAt {
		return errors.New("must provide either --before or --at")
	}
	if hasBefore && hasAt {
		return errors.New("--before and --at are mutually exclusive")
	}
	return nil
}

func allDigits(v string) bool {
	if v == "" {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
