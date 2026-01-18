package console

import (
	"strings"
	"testing"
)

func TestHighlightSyntaxSkipsLongInput(t *testing.T) {
	c := New("test")

	longInput := strings.Repeat("x", c.MaxHighlightRunes+128)
	out := c.highlightSyntax([]rune(longInput))

	if out != longInput {
		t.Fatalf("expected long input to bypass highlighting and be returned unchanged")
	}
}

func TestHighlightSyntaxShortInput(t *testing.T) {
	c := New("test")

	input := "run --flag"
	out := c.highlightSyntax([]rune(input))

	if !strings.Contains(out, "--flag") {
		t.Fatalf("expected output to contain flag text, got %q", out)
	}

	if len(out) < len(input) {
		t.Fatalf("expected highlighted output to be at least as long as input")
	}
}

func BenchmarkHighlightSyntaxShort(b *testing.B) {
	c := New("bench")
	input := []rune(strings.Repeat("echo --opt ", 8))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.highlightSyntax(input)
	}
}

func BenchmarkHighlightSyntaxLong(b *testing.B) {
	c := New("bench")
	input := []rune(strings.Repeat("x", c.MaxHighlightRunes+256))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.highlightSyntax(input)
	}
}
