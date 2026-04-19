package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// CronToStartCalendarInterval converts a 5-field cron expression to a map of
// launchd StartCalendarInterval keys (Minute, Hour, Day, Month, Weekday).
// Only non-wildcard fields are included. Complex expressions (ranges, steps,
// lists) are rejected, matching the behaviour of CronToOnCalendar.
func CronToStartCalendarInterval(cron string) (map[string]int, error) {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: %q", cron)
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	for _, f := range fields {
		if strings.ContainsAny(f, ",-/") {
			return nil, fmt.Errorf("cron expression %q contains ranges, steps, or lists — convert manually", cron)
		}
	}

	keys := map[string]string{
		"Minute":  minute,
		"Hour":    hour,
		"Day":     dom,
		"Month":   month,
		"Weekday": dow,
	}

	result := make(map[string]int)
	for key, val := range keys {
		if val == "*" {
			continue
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("invalid cron field %q: %w", val, err)
		}
		result[key] = n
	}
	return result, nil
}

// CronToOnCalendar converts a 5-field cron expression to a systemd OnCalendar value.
// supports common cases: daily, weekly, monthly, yearly.
// complex expressions (ranges, steps, lists) are not supported.
func CronToOnCalendar(cron string) (string, error) {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return "", fmt.Errorf("invalid cron expression: %q", cron)
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	for _, f := range fields {
		if strings.ContainsAny(f, ",-/") {
			return "", fmt.Errorf("cron expression %q contains ranges, steps, or lists — convert manually to OnCalendar", cron)
		}
	}

	dowMap := map[string]string{
		"0": "Sun", "1": "Mon", "2": "Tue", "3": "Wed",
		"4": "Thu", "5": "Fri", "6": "Sat",
	}

	t := fmt.Sprintf("%s:%s:00", cronPad(hour), cronPad(minute))
	date := fmt.Sprintf("*-%s-%s", cronPad(month), cronPad(dom))

	if dow != "*" {
		day, ok := dowMap[dow]
		if !ok {
			return "", fmt.Errorf("unrecognised day-of-week value: %q", dow)
		}
		return fmt.Sprintf("%s %s %s", day, date, t), nil
	}

	return fmt.Sprintf("%s %s", date, t), nil
}

func cronPad(s string) string {
	if s == "*" || len(s) >= 2 {
		return s
	}
	return "0" + s
}

func PrintDividerLine() {
	fmt.Println("---------------------------------------------")
}

func PrintDividerSpace() {
	fmt.Println("\n ")
}

type DisplayBox struct {
	w int
}

func NewDisplayBox(w int) *DisplayBox {
	return &DisplayBox{
		w: w,
	}
}

func (b *DisplayBox) BoxBorder() string {
	return strings.Repeat("─", b.w)
}

func (b *DisplayBox) BoxCenter(s string) string {
	n := utf8.RuneCountInString(s)
	pad := (b.w - n) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", b.w-pad-n)
}

// RowStr returns a formatted box row as a string without printing it.
func (b *DisplayBox) RowStr(label, value string) string {
	content := fmt.Sprintf("%s %s", label, value)
	return fmt.Sprintf("│ %-*s │\n", b.w-2, content)
}

func (b *DisplayBox) CreateRow(label, value string) {
	fmt.Print(b.RowStr(label, value))
}

// DimPrevious rewrites the last-printed box and its "Select: N" prompt in dim ANSI style.
// content is the exact string that was printed before the prompt; n is the integer the user selected.
func DimPrevious(content string, n int) {
	lines := strings.Count(content, "\n") + 1 // +1 for the "Select: N\n" line
	fmt.Printf("\033[%dA", lines)              // cursor up
	fmt.Print("\033[0J")                        // clear from cursor to end of screen
	fmt.Printf("\033[2m%sSelect: %d\n\033[0m", content, n)
}

func StringToID(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}

func CronToReadable(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return cron
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	days := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday",
		"3": "Wednesday", "4": "Thursday", "5": "Friday", "6": "Saturday",
	}
	months := map[string]string{
		"1": "January", "2": "February", "3": "March", "4": "April",
		"5": "May", "6": "June", "7": "July", "8": "August",
		"9": "September", "10": "October", "11": "November", "12": "December",
	}

	var timePart string
	switch {
	case hour == "*" && minute == "*":
		timePart = "every minute"
	case hour == "*":
		timePart = fmt.Sprintf("at minute %s of every hour", minute)
	case minute == "*":
		timePart = fmt.Sprintf("every minute past %s:00", hour)
	default:
		timePart = fmt.Sprintf("%02s:%02s", hour, minute)
	}

	switch {
	case dom == "*" && month == "*" && dow == "*":
		return fmt.Sprintf("every day at %s", timePart)
	case dom == "*" && month == "*" && dow != "*":
		return fmt.Sprintf("every %s at %s", days[dow], timePart)
	case dom != "*" && month == "*" && dow == "*":
		return fmt.Sprintf("every month on day %s at %s", dom, timePart)
	case dom != "*" && month != "*" && dow == "*":
		return fmt.Sprintf("yearly on %s %s at %s", months[month], dom, timePart)
	default:
		return cron
	}
}
