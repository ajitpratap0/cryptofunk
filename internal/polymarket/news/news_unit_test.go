package news

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"strips simple tag", "<b>bold</b>", "bold"},
		{"strips multiple tags", "<p>hello <em>world</em></p>", "hello world"},
		{"empty string", "", ""},
		{"only tags", "<br/>", ""},
		{"nested tags", "<div><span>text</span></div>", "text"},
		{"tag with attributes", `<a href="http://example.com">link</a>`, "link"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripHTML(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
