# PoC 32: a k9s-style workspace screen (#181)

## Question

Can `zero` without a command be a full-screen screen in the style of k9s
(header with context, key hints and the zero logo; a framed table; filter,
command line, describe and help views), with the libraries zero already has,
and stay testable without a person?

## Result

Yes (2026-11-28, Go 1.26.3; bubbletea v2.2.0, lipgloss v2.1.1). 29 PASS,
`fails: 0`. Rows are made up; nothing on the machine is changed.

```
Workspace: developer          <enter>  opencode   <d>      describe            ▄█████▄
Runtime:   docker-rootless    <v>      vscode     </>      filter             ▄██   ██▀
Gateway:   ok                 <u>      up         <:>      command            ██  ▄█▀██
Zero:      v0.0.0-poc         <s>      stop       <?>      help               ██▄█▀  ██
Refresh:   0                  <r>      remove     <ctrl-r> refresh            ▄█▀   ▄██
                              <n>      new        <q>      quit                ▀█████▀
┌───────────────────────────── workspaces(all)[4] ─────────────────────────────┐
│ NAME        STATUS    DIRS  TOKEN           POLICY                           │
│ apps        none      1     -               -                                │
│ cve         stopped   1     ok              default                          │
│ developer*  running   2     ok              approved                         │
└──────────────────────────────────────────────────────────────────────────────┘
```

| | result |
|---|---|
| header: context, key hints, logo as text | drawn; hints dropped below 72 columns, logo below 44, header below 16 rows |
| 100x24, 80x24, 50x18 | every line within the width |
| smaller than 20x8 | says the window is too small |
| `/` filter | narrows while typing, shown in the frame title; esc clears |
| `:` command | `q`, `help`, `all`, a workspace name jumps to it; unknown is said |
| `d` describe, `?` help | own views, esc goes back |
| enter / `v` | leaves the screen and names the command to run |
| `r` remove | the screen steps aside (`tea.ExecProcess`), the command asks on the plain terminal, the screen comes back and shows the result or the error |
| refresh | a timer message every 5 s, the keys stay responsive |
| no terminal | exit 2 (zero prints help there) |

Findings:

- The model is pure: `WindowSizeMsg` and key messages in, a string out. The
  `frame W H KEYS...` mode draws any state without a terminal, so sizes and
  views can be unit-tested in zero the same way.
- `tea.ExecProcess` keeps zero's own confirmations (`zero rm` asks for the
  name and lists unpushed work) instead of rebuilding them inside the screen.
- lipgloss `Render` writes colour codes whatever `NO_COLOR` says; bubbletea
  removes them on the way to the terminal. Tests strip them.
- The logo is the mark of `zero.png` reduced to 9x6 half-block cells.
- Nothing beyond bubbletea and lipgloss was needed; no table or text-input
  component from bubbles. Binary of the PoC: 5.6 MB.

## Sources

- bubbletea v2: `go doc charm.land/bubbletea/v2` (`View.AltScreen`,
  `ExecProcess`, `Tick`, `WindowSizeMsg`, `KeyPressMsg`),
  https://github.com/charmbracelet/bubbletea
- lipgloss v2: `go doc charm.land/lipgloss/v2`,
  https://github.com/charmbracelet/lipgloss
- k9s for the layout: https://k9scli.io/

## Limits

- Made-up rows: loading the real overview (about 1.3 s) in the background,
  and `up`, `stop`, `new` are not part of the PoC.
- The pseudo terminal check reads text with the escape codes removed; it does
  not check colours or cursor positions.
- No mouse, no resize while a command runs.

## Run

    pocs/32-k9s-style-tui/run.sh
    cd pocs/32-k9s-style-tui && go run . frame 100 24 / d e
