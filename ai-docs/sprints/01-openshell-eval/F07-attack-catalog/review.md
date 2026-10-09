# F07 review

## Step 1–4 — catalog
```bash
cd ai-docs/sprints/01-openshell-eval/F07-attack-catalog
grep -c '^| [ACX][0-9]' catalog.md     # rows
grep -c '| todo |' catalog.md          # open F08/F09 cases
grep '| gap' catalog.md                # gaps, each referenced in caveats.md
```
Expected: rows ≥ 40 (48), todo 16, gaps 6 → C08, C11, C14, C15, C17, C18 in ai-docs/caveats.md.
