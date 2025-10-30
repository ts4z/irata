package textutil

import (
	"errors"
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
