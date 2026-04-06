package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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

func (b *DisplayBox) CreateRow(label, value string) {
	content := fmt.Sprintf("%s %s", label, value)
	fmt.Printf("│ %-*s │\n", b.w-2, content)
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
