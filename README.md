# amux

A tmux popup that shows which panes have an AI agent, what that agent is doing, and
whether it is blocked on you. `prefix+W` opens a searchable tree of every session,
window, and pane; Enter jumps to the one you picked.

No daemon, no sidebar, no status bar. Hooks write a tiny state file, the popup reads it.

```
▾ PROJ-123  api  1! 2● 1✓
  ├─ ● 1 claude
  │   ├─ ! 2m   1.0   claude   feature/retry   api   why does this test fail?
  │   └─ ● 12s  1.1   claude   feature/retry   api   add the retry budget
  └─ ● 2 review
      └─ ●  8s  2.0   opencode main            api
▾ PROJ-456  web
  └─     0 tmux
      └─ ○ 1h  0.0   zsh      main            web
```

## Requirements

- tmux 3.4 or newer (`display-popup`, `after-select-pane`). On tmux 3.8+ the popup
  inherits the terminal background; older versions fall back to the configured popup style
- fzf 0.50 or newer
- OpenCode v2 for the OpenCode plugin (v1 plugins are a different API)
- Go 1.24 or newer to build

## Install

```sh
git clone https://github.com/davidalecrim1/amux
cd amux
make install          # builds to ~/.local/bin/amux
amux setup            # wires tmux, Claude Code, and OpenCode
```

`amux setup` writes `~/.config/amux/tmux.conf`, appends a `source-file` line to your
tmux config, installs the tmux focus hooks, merges eleven Claude Code hooks into
`~/.claude/settings.json` (with a timestamped backup), and writes the OpenCode plugin.
It is idempotent and supports `--dry-run`.

Then reload tmux and restart Claude Code and OpenCode.

## Keys

The picker opens in navigation mode. Typing does nothing until you ask for search.

| Key | Action |
| --- | --- |
| `j` / `k` | move down / up |
| `g` / `G` | jump to first / last |
| `h` / `l` | collapse / expand the selected session or window |
| `;` | toggle the selected session or window |
| `/` | reveal the input and enter search mode |
| `q` | quit while navigating |
| Esc | leave search mode, or quit while navigating |
| Enter | jump to the selected session, window, or pane |
| Ctrl-X | kill the selected target (asks y/N) |
| Ctrl-R | reset tracked state to idle |
| Ctrl-A | acknowledge a done marker |
| Ctrl-Y | copy `session:window.pane` |
| Ctrl-O | copy the pane's working directory |

Text is accepted only in search mode; while navigating, letters other than the bindings do
nothing. In search mode every key types normally. The preview drops the pane's background colours,
so a full-screen TUI cannot paint blocks over the popup, and each line is cut to the
preview width instead of wrapping.

## Status

| State | Glyph | Meaning |
| --- | --- | --- |
| NEEDS_INPUT | `!` | blocked on a permission prompt, question, or error |
| RUNNING | `●` | working |
| DONE_UNSEEN | `✓` | finished since you last focused the pane |
| IDLE | `○` | alive, waiting for a prompt |

Rows sort by urgency, then by last used, so stale work sinks to the bottom. Focusing a
pane clears `✓` automatically via tmux hooks.

## What it does not do

No background process, no sidebar, no status line, no unread inbox, no park/wait modes,
no notifications, no remote monitoring. See `docs/SCOPE.md`.

## When something looks wrong

```sh
amux doctor
```

It reports the binary and state paths, tracked records, orphaned records, whether the
tmux hooks are installed, and whether the Claude and OpenCode integrations are present.

A RUNNING agent with no events for a long time is usually an interrupted turn whose
`Stop` hook never fired. The age column makes it visible; `Ctrl-R` clears it.

State lives in `~/.local/state/amux/<tmux-server-pid>/<pane-id>.json` and is namespaced
by server pid, so restarting tmux never resurrects old pane state.

## Other agents

Anything can report state:

```sh
echo '{"pane_id":"%3","agent":"aider","state":"running","prompt":"fix the parser"}' | amux report
```

Valid states are `needs_input`, `running`, `done_unseen`, `idle`, and `gone` (delete).
`pane_id` defaults to `$TMUX_PANE`.

## Development

```sh
make test
make vet
make build
```
