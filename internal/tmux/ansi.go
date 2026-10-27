package tmux

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const resetSGR = "\x1b[0m"

// SanitizePreview turns captured pane output into something safe to render in a
// fixed-width preview pane: SGR foreground colours are kept, background colours
// and every non-SGR escape are dropped, and each line is cut to width visible
// cells so tmux/fzf never wraps a wide TUI across several rows.
func SanitizePreview(s string, width int) string {
	if width < 10 {
		width = 10
	}
	lines := strings.Split(s, "\n")
	trimmed := make([]string, 0, len(lines))
	hasText := make([]bool, 0, len(lines))
	for _, line := range lines {
		out, text := trimLine(line, width)
		trimmed = append(trimmed, out)
		hasText = append(hasText, text)
	}

	start, end := 0, len(trimmed)
	for start < end && !hasText[start] {
		start++
	}
	for end > start && !hasText[end-1] {
		end--
	}
	return strings.Join(trimmed[start:end], "\n")
}

func trimLine(line string, width int) (string, bool) {
	var b strings.Builder
	cells := 0
	text := false
	for i := 0; i < len(line) && cells < width; {
		if line[i] == 0x1b {
			consumed, sgr := scanEscape(line[i:])
			if consumed == 0 {
				break
			}
			if sgr != "" {
				b.WriteString(sgr)
			}
			i += consumed
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		switch {
		case r == '\r' || r == '\b' || r == 0x07:
			i += size
			continue
		case r == '\t':
			pad := 4 - cells%4
			if cells+pad > width {
				pad = width - cells
			}
			b.WriteString(strings.Repeat(" ", pad))
			cells += pad
			i += size
			continue
		case r < 0x20:
			i += size
			continue
		}
		if r != ' ' {
			text = true
		}
		b.WriteRune(r)
		cells++
		i += size
	}
	out := strings.TrimRight(b.String(), " \t")
	if strings.Contains(out, "\x1b[") {
		out += resetSGR
	}
	return out, text
}

// scanEscape consumes one escape sequence. It returns 0 when the sequence is
// truncated. The second return value is the rewritten SGR sequence, empty when
// the escape should be dropped.
func scanEscape(s string) (int, string) {
	if len(s) < 2 || s[0] != 0x1b {
		return 0, ""
	}
	switch s[1] {
	case '[':
		i := 2
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
			i++
		}
		if i >= len(s) {
			return len(s), ""
		}
		if s[i] != 'm' {
			return i + 1, ""
		}
		params := s[2:i]
		kept, ok := stripBackground(params)
		if !ok {
			return i + 1, ""
		}
		return i + 1, "\x1b[" + kept + "m"
	case ']', 'P', '^', '_':
		// OSC/DCS/PM/APC: string terminated by BEL or ST.
		i := 2
		for i < len(s) {
			if s[i] == 0x07 {
				return i + 1, ""
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2, ""
			}
			i++
		}
		return len(s), ""
	case '(', ')', '*', '+':
		if len(s) < 3 {
			return len(s), ""
		}
		return 3, ""
	default:
		return 2, ""
	}
}

// stripBackground removes SGR background parameters (40-47, 48, 49, 100-107)
// while keeping foreground and attribute parameters.
func stripBackground(params string) (string, bool) {
	if params == "" {
		return "0", true
	}
	tokens := strings.Split(params, ";")
	keep := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		n, err := strconv.Atoi(token)
		if err != nil {
			if strings.HasPrefix(token, "48:") {
				continue
			}
			keep = append(keep, token)
			continue
		}
		switch {
		case n == 48:
			if i+1 < len(tokens) {
				switch tokens[i+1] {
				case "5":
					i += 2
				case "2":
					i += 4
				default:
					i++
				}
			}
		case n == 49:
		case n >= 40 && n <= 47:
		case n >= 100 && n <= 107:
		default:
			keep = append(keep, token)
		}
	}
	if len(keep) == 0 {
		return "", false
	}
	return strings.Join(keep, ";"), true
}
