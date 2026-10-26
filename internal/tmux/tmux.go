package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Pane struct {
	SessionID    string
	SessionName  string
	SessionPath  string
	Attached     bool
	WindowID     string
	WindowIndex  string
	WindowName   string
	PaneID       string
	PaneIndex    string
	PaneCommand  string
	PanePath     string
	PanePID      int
	PaneActive   bool
	WindowActive bool
}

const sep = "\x1f"

var fields = []string{
	"#{session_id}",
	"#{session_name}",
	"#{session_path}",
	"#{session_attached}",
	"#{window_id}",
	"#{window_index}",
	"#{window_name}",
	"#{pane_id}",
	"#{pane_index}",
	"#{pane_current_command}",
	"#{pane_current_path}",
	"#{pane_pid}",
	"#{pane_active}",
	"#{window_active}",
}

func ListPanes() ([]Pane, error) {
	out, err := run("list-panes", "-a", "-F", strings.Join(fields, sep))
	if err != nil {
		return nil, err
	}
	var panes []Pane
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, sep)
		if len(parts) != len(fields) {
			continue
		}
		pid, _ := strconv.Atoi(parts[11])
		panes = append(panes, Pane{
			SessionID:    parts[0],
			SessionName:  parts[1],
			SessionPath:  parts[2],
			Attached:     truthy(parts[3]),
			WindowID:     parts[4],
			WindowIndex:  parts[5],
			WindowName:   parts[6],
			PaneID:       parts[7],
			PaneIndex:    parts[8],
			PaneCommand:  parts[9],
			PanePath:     parts[10],
			PanePID:      pid,
			PaneActive:   truthy(parts[12]),
			WindowActive: truthy(parts[13]),
		})
	}
	return panes, nil
}

func truthy(s string) bool {
	return s != "" && s != "0"
}

// ServerPID identifies the tmux server that owns the current environment.
// Pane ids are reused across server restarts, so records are namespaced by it.
func ServerPID() (int, error) {
	if v := os.Getenv("TMUX"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) >= 2 {
			if pid, err := strconv.Atoi(parts[1]); err == nil && pid > 0 {
				return pid, nil
			}
		}
	}
	out, err := run("display-message", "-p", "#{pid}")
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("cannot determine tmux server pid")
	}
	return pid, nil
}

func Jump(session, window, pane string) error {
	if session != "" {
		if _, err := run("switch-client", "-t", session); err != nil {
			return err
		}
	}
	if window != "" {
		if _, err := run("select-window", "-t", window); err != nil {
			return err
		}
	}
	if pane != "" {
		if _, err := run("select-pane", "-t", pane); err != nil {
			return err
		}
	}
	return nil
}

func KillPane(paneID string) error {
	_, err := run("kill-pane", "-t", paneID)
	return err
}

func KillWindow(target string) error {
	_, err := run("kill-window", "-t", target)
	return err
}

func KillSession(target string) error {
	_, err := run("kill-session", "-t", target)
	return err
}

// PaneFocused reports whether the pane is the active pane of an attached
// client, i.e. the user is looking at it right now.
func PaneFocused(paneID string) bool {
	out, err := run("display-message", "-p", "-t", paneID,
		"#{pane_active}"+sep+"#{window_active}"+sep+"#{session_attached}")
	if err != nil {
		return false
	}
	parts := strings.Split(strings.TrimSpace(out), sep)
	if len(parts) != 3 {
		return false
	}
	return truthy(parts[0]) && truthy(parts[1]) && parts[2] != "" && parts[2] != "0"
}

func Capture(paneID string, lines int) (string, error) {
	if lines <= 0 {
		lines = 60
	}
	out, err := run("capture-pane", "-p", "-e", "-J", "-t", paneID, "-S", fmt.Sprintf("-%d", lines*3))
	if err != nil {
		return "", err
	}
	all := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}

func ShowHooks() (string, error) {
	return run("show-hooks", "-g")
}

func SetHook(name, command string) error {
	_, err := run("set-hook", "-g", name, command)
	return err
}

// UnsetHooks removes global hooks on name whose definition contains needle.
func UnsetHooks(name, needle string) error {
	out, err := ShowHooks()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, name+"[") {
			continue
		}
		open := strings.Index(line, "[")
		closeIdx := strings.Index(line, "]")
		if open < 0 || closeIdx < open {
			continue
		}
		if !strings.Contains(line, needle) {
			continue
		}
		idx := line[open : closeIdx+1]
		if _, err := run("set-hook", "-gu", name+idx); err != nil {
			return err
		}
	}
	return nil
}

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}
