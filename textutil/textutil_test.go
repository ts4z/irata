package textutil

import "testing"

func TestWrapAttributionInNobr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		/* not handled :-(
		{
			name: "Dream",
			input: "I have a dream... -Rev. Dr. Martin Luther King, Jr.",
			expected: "I have a dream... <nobr>-Rev. Dr. Martin Luther King, Jr.</nobr>",
		},
		*/
		{
			name:     "Classic quote attribution with hyphen",
			input:    "We have nothing to fear but fear itself. -Winston Churchill",
			expected: "We have nothing to fear but fear itself. <nobr>-Winston Churchill</nobr>",
		},
		{
			name:     "Quote attribution with em dash",
			input:    "I think, therefore I am. —René Descartes",
			expected: "I think, therefore I am. <nobr>—René Descartes</nobr>",
		},
		{
			name:     "Quote attribution with tilde",
			input:    "To be or not to be. ~Shakespeare",
			expected: "To be or not to be. <nobr>~Shakespeare</nobr>",
		},
		{
			name:     "Attribution with multiple spaces before dash normalized to one",
			input:    "Quote text.  -Author Name",
			expected: "Quote text. <nobr>-Author Name</nobr>",
		},
		{
			name:     "Attribution with tab before dash normalized to one space",
			input:    "Quote text.\t-Author Name",
			expected: "Quote text. <nobr>-Author Name</nobr>",
		},
		{
			name:     "Attribution with newline before dash normalized to one space",
			input:    "Quote text.\n-Author Name",
			expected: "Quote text. <nobr>-Author Name</nobr>",
		},
		{
			name:     "Short attribution with initials",
			input:    "Famous quote. -FDR",
			expected: "Famous quote. <nobr>-FDR</nobr>",
		},
		{
			name:     "Attribution with numbers",
			input:    "Historical quote. -Author 1942",
			expected: "Historical quote. <nobr>-Author 1942</nobr>",
		},
		{
			name:     "Attribution with quotes in name",
			input:    "Some text. -Author \"The Great\"",
			expected: "Some text. <nobr>-Author \"The Great\"</nobr>",
		},
		{
			name:     "Attribution with apostrophe",
			input:    "Some text. -O'Brien",
			expected: "Some text. <nobr>-O'Brien</nobr>",
		},
		{
			name:     "Attribution with hyphenated name",
			input:    "Quote here. -Jean-Paul Sartre",
			expected: "Quote here. <nobr>-Jean-Paul Sartre</nobr>",
		},
		{
			name:     "Multi-word attribution",
			input:    "Text. -Martin Luther King Jr",
			expected: "Text. <nobr>-Martin Luther King Jr</nobr>",
		},
		{
			name:     "No match - dash at start without whitespace",
			input:    "Text-Author",
			expected: "Text-Author",
		},
		{
			name:     "No match - dash in middle with lowercase after dash",
			input:    "Some text -with dash- more text",
			expected: "Some text -with dash- more text",
		},
		{
			name:     "No match - no dash character",
			input:    "Text without attribution",
			expected: "Text without attribution",
		},
		{
			name:     "No match - invalid character after dash",
			input:    "Text. -@#$",
			expected: "Text. -@#$",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Just whitespace and dash",
			input:    " -A",
			expected: " <nobr>-A</nobr>",
		},
		{
			name:     "Attribution with trailing whitespace",
			input:    "Quote. -Author  ",
			expected: "Quote. <nobr>-Author  </nobr>",
		},
		{
			name:     "Very long attribution name",
			input:    "Quote. -Very Long Author Name With Many Words",
			expected: "Quote. <nobr>-Very Long Author Name With Many Words</nobr>",
		},
		{
			name:     "Attribution with single letter",
			input:    "Quote. -A",
			expected: "Quote. <nobr>-A</nobr>",
		},
		{
			name:     "Em dash without spaces around it",
			input:    "Text—more text",
			expected: "Text—more text",
		},
		{
			name:     "Multiple attributions (matches from last space-dash to end)",
			input:    "Quote -First. More -Second",
			expected: "Quote -First. More <nobr>-Second</nobr>",
		},
		{
			name:     "Loose dash doesn't match (space after dash)",
			input:    "Foo - bar",
			expected: "Foo - bar",
		},
		{
			name:     "Loose dash with uppercase also doesn't match (space after dash)",
			input:    "Foo - Bar",
			expected: "Foo - Bar",
		},
		{
			name:     "Leading quote",
			input:    "It is morally wrong to allow\nsuckers to keep their money.\n-\"Canada Bill\" Jones",
			expected: "It is morally wrong to allow\nsuckers to keep their money. <nobr>-\"Canada Bill\" Jones</nobr>",
		},
		{
			name:     "CRLF forever",
			input:    "It is morally wrong to allow\r\nsuckers to keep their money.\r\n-\"Canada Bill\" Jones",
			expected: "It is morally wrong to allow\r\nsuckers to keep their money. <nobr>-\"Canada Bill\" Jones</nobr>",
		},
		{
			name:     "NL leading into attribution",
			input:    "This is a quote.\n—Claude Anthropic",
			expected: "This is a quote. <nobr>—Claude Anthropic</nobr>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapAttributionInNobr(tt.input)
			if result != tt.expected {
				t.Errorf("WrapAttributionInNobr(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
