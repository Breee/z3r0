# docs — Agent Rules

User guide: Hugo site, Hextra theme. Readers: users of zero, not developers of zero.

## Layout

| Path | What |
|---|---|
| `hugo.yaml`, `go.mod` | site config, Hextra version (pinned) |
| `content/docs/*.md` | the pages; `weight` in the front matter sets the order |
| `layouts/index.html` | `/` redirects to `/docs/` |
| `adr/`, `architecture/`, `scenarios/`, `spec/`, `*.md` | reference, not part of the site; link to it, do not copy |

## Commands

```sh
hugo server --source docs                 # preview: http://localhost:1313/z3r0/
hugo --source docs --panicOnWarning       # build; must print no warning
rm -rf docs/public docs/resources         # build output, never commit
go build -o /tmp/zero-docs/zero ./cmd/zero && /tmp/zero-docs/zero   # usage text of the current build
cd docs && hugo mod get github.com/imfing/hextra@vX.Y.Z             # theme update
```

## Writing

- Short sentences, plain words. A table or a command before a paragraph.
- Front matter: `title`, `weight`, `description`.
- Links to repo files: `https://github.com/Breee/z3r0/blob/main/<path>`. Links between pages: `../page`.
- Quote output only from a run of the current build. No run: describe, do not quote.
- A limit or open risk goes on the page it belongs to and into the table in `security.md`, with its issue or ADR.
- No landing page, no custom CSS, no new shortcodes.

## GOOD, DONE, PROOF

Every change has an issue (root `AGENTS.md`). Use these per kind of change.

### zero behaviour changed (command, flag, output, capability, default)

- GOOD: the guide says what the build does. Same commit series as the code.
- DONE: `commands.md` matches the usage text; `policy.md` matches `policy.DefaultCaps` and `gitlab.Caps`; changed outputs re-quoted from a run; `security.md` tables updated when a protection or open point changed.
- PROOF:
  ```sh
  /tmp/zero-docs/zero 2>&1 | head -40          # compare with commands.md
  grep -n -A12 'DefaultCaps = ' internal/policy/policy.go
  hugo --source docs --panicOnWarning
  ```

### New or changed page

- GOOD: a user can do the task with this page alone.
- DONE: every command on the page was run and is listed in the PROOF comment, or is named there as not run with the reason; every link target exists; the page is linked from `content/docs/_index.md`.
- PROOF:
  ```sh
  hugo --source docs --panicOnWarning
  grep -o 'blob/main/[^)]*' docs/content/docs/*.md | cut -d: -f2 | sed 's#blob/main/##' | sort -u | while read -r f; do [ -e "$f" ] || echo "missing $f"; done
  ```
  The second command prints nothing.

### Site config or theme

- GOOD: looks and builds like before, except the intended change.
- DONE: version pinned in `go.mod`; all pages answer 200 in the preview; no warning.
- PROOF:
  ```sh
  hugo --source docs --panicOnWarning
  hugo server --source docs --port 1399 &
  for p in "" getting-started/ commands/ policy/ security/; do curl -s -o /dev/null -w "%{http_code} $p\n" http://localhost:1399/z3r0/docs/$p; done
  kill %1
  ```

### Close

PROOF comment with the commands above and their decisive output, what was not shown, and the findings of a review subagent that checks every claim against the code.
