# amux — Scope

Status: draft for approval
Repo: `github.com/davidalecrim1/amux`
Supersedes: `samleeney/tmux-agent-status` (architecture only; no code reuse)

## 1. Problem

`prefix + w` (tmux `choose-tree`) shows sessions, windows, and panes as undifferentiated
text. With one tmux session per Jira task and one to three panes per session, it is
impossible to tell which pane has an agent, whether that agent is working, and whether it
is blocked waiting on a permission prompt. The current hook-driven status is noisy and
sometimes sticks on the wrong state after an interrupt or a pane lifecycle change.

`tmux-agent-status` solves the state problem but adds a daemon, a persistent sidebar
pane, a status-line summary, an unread inbox, park/wait modes, and ~11.8k lines of Bash.
The sidebar is the main surface there; it is not wanted here.

## 2. What amux is

A single Go binary plus a tmux popup.

- `prefix + W` opens a temporary popup with an fzf-driven tree of the whole tmux server.
- Every pane is listed. Panes running an agent carry a status marker; plain shells do not.
- Typing filters the tree; Enter jumps to the exact session/window/pane.
- Status comes from agent hooks writing small state files. No daemon, no polling loop, no
  sidebar, no status bar.

## 3. Non-goals

Cut permanently unless re-scoped later:

- No background daemon or collector loop.
- No persistent sidebar pane, no status-right widget, no per-window glyph.
- No park/wait modes, no unread-completion inbox or history.
- No remote/SSH status, no notification sounds, no desktop toasts.
- No session deploy or worktree orchestration.
- No TPM plugin, Homebrew tap, or release pipeline. Local build and install only.
- No process-level agent detection (no pgrep/pane-scraping state machine).

## 4. UX

### 4.1 Entry point

```
bind W display-popup -E -w 90% -h 85% -T ' amux ' -b rounded '~/.local/bin/amux pick'
```

`prefix + w` is left untouched as the plain `choose-tree` fallback.

### 4.2 Layout

Every row is selectable. Box-drawing tree, sessions collapsed into headers with windows
and panes nested beneath. Everything is expanded; there is no collapse key.

```
 ▾ PROJ-123  add retry to webhook consumer            !1 ●2 ✓1
   ├─ ▾ 1 claude
   │   ├─ ●  1.1 claude  feature/retry  fix webhook timeout…   12s
   │   └─    1.2 zsh     feature/retry  ~/api
   └─ ▾ 2 review
       └─ !  2.1 claude  main           why does this fail?     2m
```

Row anatomy:

| Row | Prefix | Columns |
| --- | --- | --- |
| session | `▾` (or `▸` if attached here) | session name (bold), summary line, per-state counts right-aligned |
| window | `├─` / `└─` + `▾` | window index, window name |
| pane | `├─` / `└─` | state glyph, `window.pane`, agent or foreground command, git branch, cwd basename (`~` for `$HOME`), last prompt snippet, age right-aligned |

Session summary line is `#{session_path}` basename when it differs from the session name,
otherwise omitted.

### 4.3 Color and glyphs

Four states, one glyph each. Color does the scanning; the glyph is the fallback for
monochrome.

| State | Glyph | Color | Meaning |
| --- | --- | --- | --- |
| NEEDS_INPUT | `!` | `#f7768e` red, bold | agent is blocked on you (permission, question, error) |
| RUNNING | `●` | `#7aa2f7` blue | agent is working |
| DONE_UNSEEN | `✓` | `#9ece6a` green | agent finished since you last looked at the pane |
| IDLE | `○` | `#565f89` dim | agent is alive and waiting for a prompt |
| none | space | dim | no agent in this pane |

Session rows take the color of their most urgent descendant. Window rows are dim white.
Agent names are dim cyan, branches magenta, prompt snippets default, age dim. fzf renders
with `--ansi`.

### 4.4 Ordering

Recursive sort at every level:

1. urgency of the most urgent descendant: NEEDS_INPUT → DONE_UNSEEN → RUNNING → IDLE
2. last used, newest first

"Last used" is `max(last_focus_at, last_event_at)`: the most recent of you focusing the
pane or the agent emitting an event. Sessions and windows inherit the best value from
their children. This puts old, untouched work at the bottom without shuffling rows on
every redraw.

### 4.5 Keys

| Key | Action |
| --- | --- |
| Enter | jump to the selected session, window, or pane and close the popup |
| Ctrl-X | kill the selected pane, window, or session, after a y/N confirmation |
| Ctrl-R | reset the selected target's tracked state to IDLE |
| Ctrl-A | acknowledge DONE_UNSEEN on the selected target, clearing the marker |
| Ctrl-Y | copy the target ref (`session:window.pane`) to the clipboard |
| Ctrl-O | copy the pane's cwd to the clipboard |
| Esc | close without action |

Kill and reset on a session or window row apply to every pane beneath it.

### 4.6 Search

fzf fuzzy matches the visible row text: session name, window name, agent name, branch,
path, prompt snippet, and state glyph. `!` finds everything blocked on you. The list is
regenerated on open only; ctrl-x/ctrl-r/ctrl-a reload it in place.

A right-side preview pane (55% width) shows the last 60 lines of the highlighted pane via
`tmux capture-pane`, preserving colors. Header line: `! needs input  ● running  ✓ done  ○ idle`.

## 5. Status model

### 5.1 State ownership

Hooks own the state. amux never infers agent state from pane contents or processes.
The picker reconciles only against live tmux panes (see 5.4).

### 5.2 Claude Code

`amux hook claude <Event>` reads the hook JSON from stdin.

| Hook event | Result |
| --- | --- |
| `SessionStart` | IDLE, register agent as `claude` |
| `UserPromptSubmit` | RUNNING, store the prompt text |
| `PreToolUse`, `PostToolUse`, `PostToolUseFailure` | RUNNING, heartbeat |
| `PermissionRequest` | NEEDS_INPUT, detail = tool name |
| `PermissionDenied` | RUNNING |
| `Notification` (`permission_prompt`, `agent_needs_input`, `elicitation_dialog`) | NEEDS_INPUT |
| `Notification` (other types) | no-op |
| `Stop` | DONE_UNSEEN, unless the payload reports running `background_tasks` → RUNNING |
| `StopFailure` | NEEDS_INPUT, detail = error type |
| `SessionEnd` | delete the record |
| anything else | no-op |

Hooks are observational and must never block the agent:

- Always exit 0, swallow all errors.
- No-op when `$TMUX` or `$TMUX_PANE` is unset.
- All handlers run synchronously. Each call is a few milliseconds, and ordered
  delivery is what stops a late `PreToolUse` from overwriting a `Stop`.
- Per-event `timeout: 5`.

### 5.3 OpenCode

A generated plugin at `~/.config/opencode/plugins/amux.js`, installed by `amux setup`,
subscribing to the OpenCode event bus and piping normalized JSON into `amux report`.

| Event | Result |
| --- | --- |
| `session.status` with `status.type == "busy"` | RUNNING |
| `permission.asked` | NEEDS_INPUT |
| `permission.replied` | RUNNING |
| `tool.execute.before`, `tool.execute.after` | RUNNING, heartbeat |
| `session.idle` | DONE_UNSEEN |
| `session.error` | NEEDS_INPUT, detail = error name |
| `session.deleted` | delete the record |

Prompt text is not available on these events, so the OpenCode row shows the directory
basename and branch but no prompt snippet.

### 5.4 Staleness and reconciliation

There is no clock and no daemon, so the picker reconciles at open time:

1. Read live panes with one `tmux list-panes -a -F ...`. Any state file whose pane id is
   not in that list is deleted. This handles kill, respawn, and window moves.
2. Records are namespaced by tmux server pid (the pid field from `$TMUX`). Records from
   another server are ignored; directories for dead server pids are pruned. This handles
   a tmux server restart, which reuses pane ids.
3. If a pane still exists but its `pane_current_command` is a login shell (`sh`, `bash`,
   `zsh`, `fish`, `nu`, `dash`) and the record's last event is older than 60 seconds, the
   record is demoted to IDLE. This catches an agent that died without `SessionEnd`
   without flickering during Claude's `!` shell mode.
4. A stuck RUNNING is never auto-demoted. The row shows its age (`12m`), so a lie is
   visible, and `Ctrl-R` fixes it.

DONE_UNSEEN → IDLE happens when you focus the pane. tmux hooks (`after-select-pane`,
`after-select-window`, `client-session-changed`) call `amux focus <pane-id or
session:window>`, which clears the marker and stamps `last_focus_at`. `Ctrl-A`
acknowledges without visiting.

### 5.5 Age

Age is time since the state last changed (`updated_at`), formatted `12s`, `4m`, `2h`, `3d`.
It is the honest signal for stale state.

## 6. Architecture

```
Claude hooks ─┐
              ├─> amux report / amux hook ─> ~/.local/state/amux/<server-pid>/<pane>.json
OpenCode  ────┘                                        │
                                                       ▼
tmux list-panes -a ──────────────> amux pick (fzf) ─> jump / kill / reset / ack / copy
tmux focus hooks ─> amux focus ──> state files
```

Read path only. No goroutine runs longer than the popup, no background process survives
the command, nothing listens on a socket.

Startup budget: popup list renders in under 100 ms with 30 panes and 10 sessions. Git
branch lookup runs in parallel per pane with a 1.5 s cap; failures render `-`.

## 7. State format

`~/.local/state/amux/<server-pid>/<pane-id>.json`, one file per pane, written atomically
(temp file + rename). Different panes never contend; same-pane writes are last-wins.

```json
{
  "pane_id": "%12",
  "server_pid": 12345,
  "agent": "claude",
  "state": "needs_input",
  "session_id": "0f3c…",
  "prompt": "fix the webhook timeout",
  "detail": "Bash",
  "updated_at": "2026-10-26T16:40:12Z",
  "last_event_at": "2026-10-26T16:40:12Z",
  "last_focus_at": "2026-10-26T16:31:02Z"
}
```

`XDG_STATE_HOME` is honored. Files older than 14 days are pruned at pick time.

## 8. CLI

| Command | Purpose |
| --- | --- |
| `amux pick` | popup entrypoint: build list, run fzf, dispatch the chosen action |
| `amux list [--ansi] [--json]` | render the tree (used by fzf reload and for debugging) |
| `amux preview <target>` | capture-pane preview for fzf |
| `amux jump <target>` | switch client, window, pane |
| `amux kill <target> [--yes]` | kill pane, window, or session with confirmation |
| `amux reset <target>` | set tracked state to IDLE for the target |
| `amux ack <target>` | clear DONE_UNSEEN for the target |
| `amux copy <target> [--cwd]` | put the target ref or cwd on the clipboard |
| `amux focus <pane-id>` | tmux hook callback: mark focused, clear DONE_UNSEEN |
| `amux hook claude <Event>` | Claude Code hook adapter (stdin JSON) |
| `amux report` | normalized writer (stdin JSON), used by the OpenCode plugin |
| `amux doctor` | dump pane/state mismatches, hook installation status, binary path |
| `amux setup [--dry-run]` | install tmux bindings/hooks, Claude hooks, OpenCode plugin |

Targets: `%pane`, `session:window.pane`, `session:window`, `session`. Rows carry an opaque
target id in a tab-separated field that fzf hides.

## 9. Setup

`make install` builds to `~/.local/bin/amux`. `amux setup` is idempotent and backs up
before editing:

1. Writes `~/.config/amux/tmux.conf`:

```tmux
bind W display-popup -E -w 90% -h 85% -T ' amux ' -b rounded '~/.local/bin/amux pick'
set-hook -g after-select-pane   'run-shell -b "~/.local/bin/amux focus #{pane_id}"'
set-hook -g after-select-window 'run-shell -b "~/.local/bin/amux focus #{pane_id}"'
set-hook -g client-session-changed 'run-shell -b "~/.local/bin/amux focus #{pane_id}"'
```

and appends `source-file ~/.config/amux/tmux.conf` to `~/.config/tmux/tmux.conf` if the
line is absent. Existing `amux` hooks are removed first so re-running setup never
duplicates them.

2. Merges the Claude Code hooks into `~/.claude/settings.json`, adding to each event's
   array without touching existing handlers. Timestamped backup at
   `~/.claude/settings.json.amux-<ts>.bak`. Detects an existing amux handler by command
   path and skips it.

3. Writes the OpenCode plugin to `~/.config/opencode/plugins/amux.js`, refusing to
   overwrite a file it did not generate (marker comment on line 1).

4. Prints what changed and tells you to run `tmux source-file`, restart Claude Code, and
   restart OpenCode.

`--dry-run` prints every file write and settings mutation without applying it.

## 10. Repo layout

```
amux/
  cmd/amux/main.go            # command dispatch
  internal/cli/               # pick, list, jump, kill, reset, ack, copy, focus, doctor
  internal/tmux/tmux.go       # list-panes, capture, jump, kill, hooks
  internal/state/store.go     # atomic read/write/prune
  internal/agent/claude.go    # Claude hook event → state transition
  internal/render/tree.go     # rows, colors, ordering
  internal/picker/fzf.go      # fzf invocation, keybinds, reload
  internal/setup/             # tmux/claude/opencode installation
  docs/SCOPE.md
  Makefile                    # build, install, test, fmt, vet
  README.md
```

## 11. Acceptance criteria

1. `prefix+W` opens the popup in under 100 ms with 10 sessions and 30 panes.
2. Submitting a Claude prompt makes that pane RUNNING before the first tool call.
3. A Claude permission prompt makes it NEEDS_INPUT before the dialog appears.
4. `Stop` makes it DONE_UNSEEN; focusing the pane makes it IDLE.
5. Esc-interrupt leaves RUNNING, the age column grows, and Ctrl-R recovers it.
6. Killing a pane removes its row on the next open; renaming a session or window is
   reflected immediately because names are read live.
7. Pane ids reused after a tmux server restart never inherit old state.
8. `ps` shows no amux process when the popup is closed.
9. Plain shells are listed, selectable, and visually quieter than agent panes.
10. Fuzzy search matches session name, window name, branch, path, and prompt snippet.
11. `amux setup --dry-run` changes nothing; running it twice produces no duplicates.
12. `go test ./...` covers state transitions, ordering, rendering, and target parsing.

## 12. Open items to verify during build

Defaults below are what I will implement unless you object:

1. **tmux hook context** — confirm `#{pane_id}` resolves to the new active pane inside
   `after-select-window` and `client-session-changed`; otherwise fall back to
   `#{session_name}:#{window_index}` plus a lookup.
2. **OpenCode payload fields** — confirm `event.properties.sessionID` and
   `status.type` shapes with a temporary logging plugin; adjust the adapter.
3. **Claude hooks** — verify every handler fires and that ordered delivery keeps `Stop`
   from being overwritten by a trailing tool event.
4. **Claude `!` shell mode** — check whether `pane_current_command` flips to the shell
   while the agent is alive; if it does often, drop rule 5.4.3 entirely.
5. **Clipboard tool** — `pbcopy` on macOS, `wl-copy`/`xclip` on Linux; no-op with a
   message when neither exists.
