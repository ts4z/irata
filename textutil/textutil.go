package textutil

import (
	"strings"
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
