package textutil

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func WrapLinesInNOBR(input string) string {
	sb := strings.Builder{}
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		sb.WriteString("<nobr>")
		sb.WriteString(line)
		sb.WriteString("</nobr> ")
	}
	return sb.String()
}

func JoinNLNL(input []string) string {
	return strings.Join(input, "\n\n")
}

// Parse MM:SS or HH:MM:SS format into time.Duration.
func ParseDuration(s string) (time.Duration, error) {
	var hh, mm, ss string
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		hh, mm, ss = parts[0], parts[1], parts[2]
	} else if len(parts) == 2 {
		hh, mm, ss = "0", parts[0], parts[1]
	} else {
		return 0, errors.New("invalid HH:MM:SS format")
	}

	hours, err := strconv.Atoi(hh)
	if err != nil {
		return 0, errors.New("can't parse hours")
	}
	mins, err := strconv.Atoi(mm)
	if err != nil {
		return 0, errors.New("can't parse minutes")
	}
	secs, err := strconv.Atoi(ss)
	if err != nil {
		return 0, errors.New("can't parse seconds")
	}

	return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute + time.Duration(secs)*time.Second, nil
}

// formatPlace converts a numeric place (1, 2, 3, ...) to a string ("1st", "2nd", "3rd", ...).
func FormatPlace(place int) string {
	suffix := "th"
	if place%100 < 11 || place%100 > 13 {
		switch place % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", place, suffix)
}

// Join concatenates the elements of a string slice with a separator.
func Join(elems []string, sep string) string {
	return strings.Join(elems, sep)
}

// JoinInts concatenates the elements of an int slice with a separator.
func JoinInts(elems []int, sep string) string {
	strs := make([]string, len(elems))
	for i, v := range elems {
		strs[i] = strconv.Itoa(v)
	}
	return strings.Join(strs, sep)
}
