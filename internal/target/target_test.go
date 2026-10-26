package target

import "testing"

func TestRoundTrip(t *testing.T) {
	cases := []Target{
		{Kind: Session, Session: "PROJ-123"},
		{Kind: Window, Session: "PROJ-123", Window: "2"},
		{Kind: Pane, PaneID: "%12"},
		{Kind: Window, Session: "a:b", Window: "0"},
		{Kind: Session, Session: "with space"},
	}
	for _, want := range cases {
		got, err := Parse(Ref(want))
		if err != nil {
			t.Fatalf("Parse(%q): %v", Ref(want), err)
		}
		if got.Kind != want.Kind || got.Session != want.Session || got.Window != want.Window || got.PaneID != want.PaneID {
			t.Fatalf("Parse(%q) = %+v, want %+v", Ref(want), got, want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", "x:1", "s:", "w:", "w:session", "w:session:", "p:"} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) should fail", in)
		}
	}
}
