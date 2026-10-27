package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/davidalecrim1/amux/internal/target"
)

func (a *App) pick(args []string) int {
	s, err := a.load(true)
	if err != nil {
		return a.fail("%v", err)
	}
	rows := a.renderRows(s, true)
	if len(rows) == 0 {
		fmt.Fprintln(a.Stderr, "amux: no tmux panes")
		return 0
	}
	exe, err := os.Executable()
	if err != nil {
		return a.fail("%v", err)
	}
	bin := shellQuote(exe)

	fzfArgs := []string{
		"--ansi",
		"--layout=reverse",
		"--height=100%",
		"--no-sort",
		"--disabled",
		"--color=bg+:-1",
		"--delimiter=\t",
		"--with-nth=1",
		"--nth=1",
		"--header= ! needs input   ● running   ✓ done   ○ idle      j/k move   / search   ^X kill   ^R reset   ^A ack ",
		"--header-first",
		"--pointer=▌",
		"--marker=┃",
		"--prompt=amux> ",
		"--preview=" + bin + " preview {2}",
		"--preview-window=right:55%",
		"--bind", `j:transform:[ "$FZF_INPUT_STATE" = disabled ] && echo down || echo "put(j)"`,
		"--bind", `k:transform:[ "$FZF_INPUT_STATE" = disabled ] && echo up || echo "put(k)"`,
		"--bind", `g:transform:[ "$FZF_INPUT_STATE" = disabled ] && echo first || echo "put(g)"`,
		"--bind", `G:transform:[ "$FZF_INPUT_STATE" = disabled ] && echo last || echo "put(G)"`,
		"--bind", `/:transform:[ "$FZF_INPUT_STATE" = disabled ] && echo "enable-search+change-prompt(search> )" || echo "put(/)"`,
		"--bind", `esc:transform:[ "$FZF_INPUT_STATE" = enabled ] && echo "disable-search+clear-query+change-prompt(amux> )" || echo abort`,
		"--bind=ctrl-x:execute(" + bin + " kill {2})+reload(" + bin + " list --ansi)",
		"--bind=ctrl-r:execute-silent(" + bin + " reset {2})+reload(" + bin + " list --ansi)",
		"--bind=ctrl-a:execute-silent(" + bin + " ack {2})+reload(" + bin + " list --ansi)",
		"--bind=ctrl-y:execute-silent(" + bin + " copy {2})",
		"--bind=ctrl-o:execute-silent(" + bin + " copy --cwd {2})",
	}

	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = strings.NewReader(strings.Join(rows, "\n") + "\n")
	cmd.Stderr = a.Stderr
	var out strings.Builder
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			switch ee.ExitCode() {
			case 130, 1:
				return 0
			}
		}
		if _, ok := err.(*exec.ExitError); ok {
			return 0
		}
		return a.fail("fzf: %v", err)
	}
	sel := strings.TrimRight(out.String(), "\n")
	if sel == "" {
		return 0
	}
	t, err := parseSelection(sel)
	if err != nil {
		return a.fail("%v", err)
	}
	return a.jump(s, t)
}

func parseSelection(line string) (target.Target, error) {
	idx := strings.LastIndex(line, "\t")
	if idx < 0 {
		return target.Target{}, fmt.Errorf("unexpected fzf output")
	}
	return target.Parse(line[idx+1:])
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t'\"$`\\!*?()[]{};&|<>#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
