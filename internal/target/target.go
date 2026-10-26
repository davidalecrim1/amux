package target

import (
	"fmt"
	"strings"
)

type Kind int

const (
	Session Kind = iota
	Window
	Pane
)

type Target struct {
	Kind    Kind
	Session string
	Window  string
	PaneID  string
}

// Ref is the opaque id carried in a hidden column of each row.
func Ref(t Target) string {
	switch t.Kind {
	case Session:
		return "s:" + t.Session
	case Window:
		return "w:" + t.Session + ":" + t.Window
	default:
		return "p:" + t.PaneID
	}
}

func Parse(ref string) (Target, error) {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "s:"):
		name := strings.TrimPrefix(ref, "s:")
		if name == "" {
			return Target{}, fmt.Errorf("empty session target")
		}
		return Target{Kind: Session, Session: name}, nil
	case strings.HasPrefix(ref, "w:"):
		rest := strings.TrimPrefix(ref, "w:")
		idx := strings.LastIndex(rest, ":")
		if idx <= 0 || idx == len(rest)-1 {
			return Target{}, fmt.Errorf("bad window target %q", ref)
		}
		return Target{Kind: Window, Session: rest[:idx], Window: rest[idx+1:]}, nil
	case strings.HasPrefix(ref, "p:"):
		pane := strings.TrimPrefix(ref, "p:")
		if pane == "" {
			return Target{}, fmt.Errorf("empty pane target")
		}
		return Target{Kind: Pane, PaneID: pane}, nil
	}
	return Target{}, fmt.Errorf("bad target %q", ref)
}

func (t Target) String() string {
	switch t.Kind {
	case Session:
		return t.Session
	case Window:
		return t.Session + ":" + t.Window
	default:
		return t.PaneID
	}
}

func (t Target) Describe() string {
	switch t.Kind {
	case Session:
		return "session " + t.Session
	case Window:
		return "window " + t.Session + ":" + t.Window
	default:
		return "pane " + t.PaneID
	}
}
