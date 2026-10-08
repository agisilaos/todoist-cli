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
		mins, err := strconv.Atoi(raw)
		if err != nil || mins <= 0 {
			return 0, errors.New("duration must be positive")
		}
		return mins, nil
	}
	re := regexp.MustCompile(`^(\d+)\s*(hours?|hrs?|h|minutes?|mins?|m|s)`)
	maxMinutes := int(^uint(0) >> 1)
	total, seconds := 0, 0
	for raw != "" {
		m := re.FindStringSubmatch(raw)
		if m == nil {
			return 0, fmt.Errorf("invalid duration: %s", value)
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("duration is too large: %s", value)
		}
		switch m[2] {
		case "h", "hr", "hrs", "hour", "hours":
			if n > maxMinutes/60 {
				return 0, fmt.Errorf("duration is too large: %s", value)
			}
			n *= 60
		case "m", "min", "mins", "minute", "minutes":
		case "s":
			seconds += n % 60
			n = n/60 + seconds/60
			seconds %= 60
		}
		if n > maxMinutes-total {
			return 0, fmt.Errorf("duration is too large: %s", value)
		}
		total += n
		raw = strings.TrimSpace(raw[len(m[0]):])
	}
	// Todoist offsets use whole minutes. Round the complete duration up once.
	if seconds > 0 {
		if total == maxMinutes {
			return 0, fmt.Errorf("duration is too large: %s", value)
		}
		total++
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
