package textutil

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var nobrAttributionRE = regexp.MustCompile(
	// must start with some leading space
	`(?:\s|\n|\r)+` +
		// need some kind of hyphen-looking thing
		`((?:-|—|~)` +
		// need a leading uppercase character or something quotey
		`(?:\p{Lu}|"|&#34;)` +
		// Now we need a bunch of nickname/name characters.  This is a little problematic
		// because names can have dots and commas, and we aren't currently handling
		// that case.  Sorry, Rev. Dr. Martin Luther King, Jr.  This is complicated
		// by quotes that have additional phrases for context (like mentioning
		// the movie the quote came from).  We could probably mitigate this by looking
		// for repeated capitalized words (until you think about e.e. cummings but I think
		// that's a special case even I can let go).
		`(?:\pL|\pN|"|&#34;|'| |-)*)` +
		// must be at end of string
		`$`)

// WrapAttributionInNobr wraps quote attributions at the end of strings in a <nobr> tag.
// This prevents line breaks in attributions like "Quote text. -Winston Churchill"
//
// This is hairy becasue it will mostly work on HTML-escaped text, and will probably get
// worse as it works on more text in the future.
func WrapAttributionInNobr(s string) string {
	return nobrAttributionRE.ReplaceAllString(s, " <nobr>$1</nobr>")
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
