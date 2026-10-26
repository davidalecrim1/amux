package cli

import (
	"encoding/json"
	"io"
	"os"

	"github.com/davidalecrim1/amux/internal/agent"
	"github.com/davidalecrim1/amux/internal/state"
	"github.com/davidalecrim1/amux/internal/tmux"
)

func (a *App) hookCmd(args []string) int {
	if len(args) < 2 || args[0] != "claude" {
		return 0
	}
	event := args[1]

	var payload map[string]any
	if dec := json.NewDecoder(io.LimitReader(os.Stdin, 4<<20)); dec.Decode(&payload) != nil {
		payload = map[string]any{}
	}

	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return 0
	}
	pid, err := tmux.ServerPID()
	if err != nil {
		return 0
	}
	now := a.Now()

	rec, err := a.Store.Get(pid, paneID)
	if err != nil {
		return 0
	}
	if rec == nil {
		if event == "SessionEnd" {
			return 0
		}
		rec = &state.Record{
			PaneID:    paneID,
			ServerPID: pid,
			Agent:     "claude",
			State:     state.Idle,
		}
	}

	res := agent.ApplyClaude(event, payload, rec, now)
	if res.Delete {
		_ = a.Store.Delete(pid, paneID)
		return 0
	}
	if res.Changed {
		_ = a.Store.Save(*rec)
	}
	return 0
}

type reportInput struct {
	PaneID    string `json:"pane_id"`
	Agent     string `json:"agent"`
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
}

func (a *App) reportCmd(args []string) int {
	var in reportInput
	if dec := json.NewDecoder(io.LimitReader(os.Stdin, 4<<20)); dec.Decode(&in) != nil {
		return a.fail("report: invalid JSON on stdin")
	}
	if in.PaneID == "" {
		in.PaneID = os.Getenv("TMUX_PANE")
	}
	if in.PaneID == "" {
		return 0
	}
	pid, err := tmux.ServerPID()
	if err != nil {
		return 0
	}
	if in.State == "gone" || in.State == "end" {
		_ = a.Store.Delete(pid, in.PaneID)
		return 0
	}
	st := state.State(in.State)
	if !st.Valid() {
		return a.fail("report: unknown state %q", in.State)
	}
	now := a.Now()
	rec, err := a.Store.Get(pid, in.PaneID)
	if err != nil {
		return a.fail("%v", err)
	}
	if rec == nil {
		rec = &state.Record{PaneID: in.PaneID, ServerPID: pid, Agent: in.Agent, State: st}
	}
	if in.Agent != "" {
		rec.Agent = in.Agent
	}
	if in.SessionID != "" {
		rec.SessionID = in.SessionID
	}
	if in.Prompt != "" {
		rec.Prompt = in.Prompt
	}
	rec.Detail = in.Detail
	if rec.State != st || rec.UpdatedAt.IsZero() {
		rec.State = st
		rec.UpdatedAt = now
	}
	rec.LastEvent = now
	if err := a.Store.Save(*rec); err != nil {
		return a.fail("%v", err)
	}
	return 0
}
