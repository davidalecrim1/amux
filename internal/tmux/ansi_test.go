package tmux

import (
	"strings"
	"testing"
)

func TestSanitizeDropsBackgrounds(t *testing.T) {
	in := "\x1b[48;2;30;30;30m  hello  \x1b[0m \x1b[48;5;236mworld\x1b[0m"
	got := SanitizePreview(in, 40)
	if strings.Contains(got, "48;") {
		t.Fatalf("background survived: %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Fatalf("text lost: %q", got)
	}
}

func TestSanitizeKeepsForeground(t *testing.T) {
	in := "\x1b[38;2;1;2;3mred\x1b[0m"
	got := SanitizePreview(in, 40)
	if !strings.Contains(got, "\x1b[38;2;1;2;3m") {
		t.Fatalf("foreground lost: %q", got)
	}
}

func TestSanitizeMixedParams(t *testing.T) {
	in := "\x1b[1;48;5;236;38;5;250mX\x1b[0m"
	got := SanitizePreview(in, 40)
	if strings.Contains(got, "48") {
		t.Fatalf("background code survived: %q", got)
	}
	if !strings.Contains(got, "\x1b[1;38;5;250m") {
		t.Fatalf("expected bold + fg, got %q", got)
	}
}

func TestSanitizeDropsClearAndCursorMoves(t *testing.T) {
	in := "a\x1b[2Jb\x1b[Hc\x1b[10;20Hd\x1b[?25l"
	got := SanitizePreview(in, 40)
	if got != "abcd" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeDropsOSC(t *testing.T) {
	in := "\x1b]0;title\x07text\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\"
	got := SanitizePreview(in, 40)
	if strings.Contains(got, "\x1b]") {
		t.Fatalf("OSC survived: %q", got)
	}
	if !strings.Contains(got, "text") || !strings.Contains(got, "link") {
		t.Fatalf("text lost: %q", got)
	}
}

func TestSanitizeTruncatesWidth(t *testing.T) {
	in := strings.Repeat("a", 100)
	got := SanitizePreview(in, 20)
	if len(got) != 20 {
		t.Fatalf("width = %d, want 20", len(got))
	}
}

func TestSanitizeTrimsLeadingAndTrailingBlankLines(t *testing.T) {
	in := "\n   \n\nfirst\nsecond\n\n   \n"
	got := SanitizePreview(in, 40)
	if got != "first\nsecond" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitizeTabsExpand(t *testing.T) {
	got := SanitizePreview("a\tb", 40)
	if got != "a   b" {
		t.Fatalf("got %q", got)
	}
}
