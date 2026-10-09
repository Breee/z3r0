# F07 — Attack-vector catalog
Status: done
Description: One table of all attack vectors we test against the sandbox layer: arch-defined (§14.1–14.13, §12.6), common (OWASP LLM / container escape), creative. Drives F02/F03/F08/F09.
Goal: Every vector has ID, layer, expected control, test case, status.
Plan:
1. Extract arch vectors (§14, §12.6) → `catalog.md` rows A01…, mark which are sandbox-layer (testable now) vs Z3R0/server-side (later sprint).
2. Add common vectors C-xx: container escape, proxy bypass, DNS exfil, resource exhaustion, secret in env/proc/core dumps, supply chain, prompt injection via tool output.
3. Add creative vectors X-xx (brainstorm, see F08 seeds).
4. Map each row to an existing test (F02/F03 case name) or a F08/F09 todo.
DONE: catalog.md exists; every row has layer + expected control + test ref or "later sprint: reason".
GOOD: ≤1 line per row; gaps (no control) are copied to caveats.md.
Proof: `grep -c '^| ' catalog.md` ≥ 40 rows; `grep -c 'todo' catalog.md` matches open F08/F09 cases.
Log:
- Steps 1–4: catalog.md 48 rows (18 A, 20 C, 10 X); 18 pass, 16 todo, 6 gap, 8 later; gaps → C17, C18. DONE.
