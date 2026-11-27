# PoC 31: first-run questions and workspace list with Charm v2 (#185)

## Question

Can the first run of zero in a repository ask for hosts and permissions with
detected, pre-selected defaults, and can `zero` without a command show a
workspace list, with Charm's libraries, while scripts and pipes stay
question-free?

## Result

Yes (2026-11-28, Go 1.26.3; bubbletea v2.2.0, huh v2.0.3, lipgloss v2.1.1).
12 PASS, `fails: 0`.

| | result |
|---|---|
| hosts detected from `go.mod`, `package.json` (also `requirements.txt`/`pyproject.toml`, `Cargo.toml`) | pre-selected in a multi-select |
| permissions | safe set pre-selected; `ci.run`, `gitlab.mr.create` off, explained |
| Enter, Enter, Enter | defaults written unchanged |
| `gitlab.mr.create` without `ci.run` | refused inside the form, with the reason |
| answer no | nothing written, exit 1 |
| no terminal (`</dev/null`, pipe) | no questions, defaults, exit 0 |
| workspace list | arrow keys / j k, one key per action, q leaves |
| workspace list without a terminal | exit 2 (zero prints help there) |

Findings:

- huh forms and a bubbletea list work on a pseudo terminal, so both can be
  tested without a person (`ptyrun.py`).
- The form goes to stderr, the result to stdout; pipes stay clean.
- Cost: one binary of 6.8 MB for the PoC (zero today: 11.3 MB) and about
  25 indirect modules (Charm's own, `x/sys`, `x/sync`, clipboard, runewidth).
- Charm v2 modules live at `charm.land/...`, not `github.com/charmbracelet`.

## Sources

- https://pkg.go.dev/charm.land/huh/v2 (`NewForm`, `NewMultiSelect`, `NewConfirm`, `Validate`, `WithOutput`, `ErrUserAborted`)
- https://pkg.go.dev/charm.land/bubbletea/v2 (`Model`, `View`, `KeyPressMsg`, `NewProgram`)
- https://pkg.go.dev/charm.land/lipgloss/v2
- `go doc` of the three modules at the versions above

## Limits

- The list shows fixed rows; reading zero's overview (`ls.go`) and running the
  actions is the build.
- Not checked: Windows, screen readers (`WithAccessible`), `NO_COLOR`.
- The hosts per preset are a first set, not checked against real installs.

## Run

    pocs/31-cli-first-run-and-tui/run.sh
