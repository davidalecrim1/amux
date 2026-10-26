package render

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/davidalecrim1/amux/internal/state"
	"github.com/davidalecrim1/amux/internal/target"
	"github.com/davidalecrim1/amux/internal/tmux"
)

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	red     = "\x1b[38;2;247;118;142m"
	blue    = "\x1b[38;2;122;162;247m"
	green   = "\x1b[38;2;158;206;106m"
	dim     = "\x1b[38;2;86;95;137m"
	cyan    = "\x1b[38;2;125;207;255m"
	magenta = "\x1b[38;2;187;154;247m"
	white   = "\x1b[38;2;192;202;245m"
)

type Options struct {
	Now      time.Time
	Branches map[string]string
	ANSI     bool
}

type paneView struct {
	pane     tmux.Pane
	rec      *state.Record
	branch   string
	lastUsed time.Time
	rank     int
}

type windowView struct {
	index    string
	name     string
	panes    []paneView
	hasAgent bool
	rank     int
	lastUsed time.Time
}

type sessionView struct {
	name     string
	path     string
	attached bool
	windows  []windowView
	counts   map[state.State]int
	hasAgent bool
	rank     int
	lastUsed time.Time
}

// Rows renders the tree and returns fzf input lines. Each line is the visible
// row, a tab, and the opaque target ref.
func Rows(panes []tmux.Pane, records map[string]state.Record, opt Options) []string {
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	sessions := group(panes, records, opt)

	var out []string
	for _, s := range sessions {
		out = append(out, renderSession(s, opt)+"\t"+target.Ref(target.Target{
			Kind: target.Session, Session: s.name,
		}))
		for wi, w := range s.windows {
			lastWin := wi == len(s.windows)-1
			out = append(out, renderWindow(s, w, lastWin, opt)+"\t"+target.Ref(target.Target{
				Kind: target.Window, Session: s.name, Window: w.index,
			}))
			for pi, p := range w.panes {
				lastPane := pi == len(w.panes)-1
				out = append(out, renderPane(s, w, p, lastWin, lastPane, opt)+"\t"+target.Ref(target.Target{
					Kind: target.Pane, PaneID: p.pane.PaneID,
				}))
			}
		}
	}
	return out
}

func group(panes []tmux.Pane, records map[string]state.Record, opt Options) []sessionView {
	bySession := map[string]*sessionView{}
	var order []string
	for _, p := range panes {
		s, ok := bySession[p.SessionName]
		if !ok {
			s = &sessionView{name: p.SessionName, path: p.SessionPath, attached: p.Attached, counts: map[state.State]int{}, rank: state.Idle.Rank()}
			bySession[p.SessionName] = s
			order = append(order, p.SessionName)
		}
		wIdx := -1
		for i := range s.windows {
			if s.windows[i].index == p.WindowIndex {
				wIdx = i
				break
			}
		}
		if wIdx < 0 {
			s.windows = append(s.windows, windowView{index: p.WindowIndex, name: p.WindowName, rank: state.Idle.Rank()})
			wIdx = len(s.windows) - 1
		}
		pv := paneView{pane: p, rank: state.Idle.Rank(), branch: opt.Branches[p.PaneID]}
		if rec, ok := records[p.PaneID]; ok {
			r := rec
			pv.rec = &r
			pv.rank = rec.State.Rank()
			pv.lastUsed = rec.LastUsed()
			s.counts[rec.State]++
			if !s.hasAgent || pv.rank < s.rank {
				s.rank = pv.rank
			}
			s.hasAgent = true
			if pv.lastUsed.After(s.lastUsed) {
				s.lastUsed = pv.lastUsed
			}
			if !s.windows[wIdx].hasAgent || pv.rank < s.windows[wIdx].rank {
				s.windows[wIdx].rank = pv.rank
				s.windows[wIdx].hasAgent = true
			}
			if pv.lastUsed.After(s.windows[wIdx].lastUsed) {
				s.windows[wIdx].lastUsed = pv.lastUsed
			}
		}
		s.windows[wIdx].panes = append(s.windows[wIdx].panes, pv)
	}

	out := make([]sessionView, 0, len(order))
	for _, name := range order {
		s := *bySession[name]
		for i := range s.windows {
			w := &s.windows[i]
			sort.SliceStable(w.panes, func(a, b int) bool {
				if w.panes[a].rank != w.panes[b].rank {
					return w.panes[a].rank < w.panes[b].rank
				}
				if !w.panes[a].lastUsed.Equal(w.panes[b].lastUsed) {
					return w.panes[a].lastUsed.After(w.panes[b].lastUsed)
				}
				return indexOf(w.panes[a].pane) < indexOf(w.panes[b].pane)
			})
		}
		sort.SliceStable(s.windows, func(a, b int) bool {
			x, y := s.windows[a], s.windows[b]
			if x.rank != y.rank {
				return x.rank < y.rank
			}
			if !x.lastUsed.Equal(y.lastUsed) {
				return x.lastUsed.After(y.lastUsed)
			}
			return indexOfStr(x.index) < indexOfStr(y.index)
		})
		out = append(out, s)
	}
	sort.SliceStable(out, func(a, b int) bool {
		x, y := out[a], out[b]
		if x.rank != y.rank {
			return x.rank < y.rank
		}
		if !x.lastUsed.Equal(y.lastUsed) {
			return x.lastUsed.After(y.lastUsed)
		}
		return x.name < y.name
	})
	return out
}

func renderSession(s sessionView, opt Options) string {
	arrow := "▾"
	if s.attached {
		arrow = "▸"
	}
	nameColor := white
	if s.hasAgent && s.rank != state.Idle.Rank() {
		nameColor = stateColor(state.State(rankState(s.rank)))
	}
	var b strings.Builder
	b.WriteString(paint(opt, arrow+" ", dim))
	b.WriteString(paint(opt, s.name, bold+nameColor))
	if base := pathBase(s.path); base != "" {
		b.WriteString("  ")
		b.WriteString(paint(opt, base, dim))
	}
	if counts := renderCounts(s.counts, opt); counts != "" {
		b.WriteString("  ")
		b.WriteString(counts)
	}
	return b.String()
}

func renderWindow(s sessionView, w windowView, last bool, opt Options) string {
	branch := "  ├─ "
	if last {
		branch = "  └─ "
	}
	var b strings.Builder
	b.WriteString(paint(opt, branch, dim))
	if w.hasAgent {
		b.WriteString(paint(opt, glyph(state.State(rankState(w.rank)))+" ", stateColor(state.State(rankState(w.rank)))))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(paint(opt, w.index, bold+white))
	b.WriteString(" ")
	b.WriteString(paint(opt, w.name, dim))
	return b.String()
}

func renderPane(s sessionView, w windowView, p paneView, lastWin, lastPane bool, opt Options) string {
	indent := "  │   "
	if lastWin {
		indent = "      "
	}
	branch := "├─ "
	if lastPane {
		branch = "└─ "
	}
	var b strings.Builder
	b.WriteString(paint(opt, indent+branch, dim))

	if p.rec != nil {
		col := stateColor(p.rec.State)
		b.WriteString(paint(opt, glyph(p.rec.State), bold+col))
		b.WriteString(" ")
		b.WriteString(paint(opt, pad(FormatAge(opt.Now.Sub(p.rec.UpdatedAt)), 4), dim))
		b.WriteString(" ")
	} else {
		b.WriteString("       ")
	}

	b.WriteString(paint(opt, pad(w.index+"."+p.pane.PaneIndex, 5), dim))
	b.WriteString(" ")
	b.WriteString(paint(opt, pad(truncate(agentName(p), 8), 8), cyan))
	b.WriteString(" ")
	b.WriteString(paint(opt, pad(truncate(branchName(p), 16), 16), magenta))
	b.WriteString(" ")
	b.WriteString(paint(opt, pad(truncate(pathBase(p.pane.PanePath), 14), 14), dim))

	if extra := paneExtra(p); extra != "" {
		b.WriteString("  ")
		b.WriteString(paint(opt, truncate(extra, 56), white))
	}
	return strings.TrimRight(b.String(), " ")
}

func paneExtra(p paneView) string {
	if p.rec == nil {
		return ""
	}
	if p.rec.Prompt != "" {
		return p.rec.Prompt
	}
	if p.rec.State == state.NeedsInput && p.rec.Detail != "" {
		return p.rec.Detail
	}
	return ""
}

func agentName(p paneView) string {
	if p.rec != nil && p.rec.Agent != "" {
		return p.rec.Agent
	}
	return p.pane.PaneCommand
}

func branchName(p paneView) string {
	if p.branch == "" {
		return "-"
	}
	return p.branch
}

func renderCounts(counts map[state.State]int, opt Options) string {
	var parts []string
	for _, st := range []state.State{state.NeedsInput, state.DoneUnseen, state.Running, state.Idle} {
		n := counts[st]
		if n == 0 {
			continue
		}
		parts = append(parts, paint(opt, fmt.Sprintf("%d%s", n, glyph(st)), stateColor(st)))
	}
	return strings.Join(parts, " ")
}

func glyph(s state.State) string {
	switch s {
	case state.NeedsInput:
		return "!"
	case state.Running:
		return "●"
	case state.DoneUnseen:
		return "✓"
	default:
		return "○"
	}
}

func stateColor(s state.State) string {
	switch s {
	case state.NeedsInput:
		return red
	case state.Running:
		return blue
	case state.DoneUnseen:
		return green
	default:
		return dim
	}
}

func rankState(rank int) state.State {
	switch rank {
	case 0:
		return state.NeedsInput
	case 1:
		return state.DoneUnseen
	case 2:
		return state.Running
	default:
		return state.Idle
	}
}

func paint(opt Options, s, color string) string {
	if !opt.ANSI || s == "" {
		return s
	}
	return color + s + reset
}

func pathBase(path string) string {
	if path == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+"/") {
			path = "~" + strings.TrimPrefix(path, home)
		}
	}
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		return path
	}
	base := trimmed
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		base = trimmed[i+1:]
	}
	if base == "" || base == "~" {
		return "~"
	}
	return base
}

func FormatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\t", " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

func pad(s string, n int) string {
	w := utf8.RuneCountInString(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func indexOf(p tmux.Pane) int {
	v, _ := strconv.Atoi(p.PaneIndex)
	return v
}

func indexOfStr(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
