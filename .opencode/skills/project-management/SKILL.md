---
name: project-management
description: Use before creating, updating, starting, or closing any work item in zero (Breee/z3r0) — issues, bugs, features, enhancements, security risks, caveats, upstream reports, sprints. Defines issue types, required fields, GitHub Project fields, and the GOOD/DONE/PROOF/REVIEW workflow.
---

# Project management for zero

Source of truth: GitHub issues in `Breee/z3r0` + GitHub Project **zero** (`gh project … 2 --owner Breee`, https://github.com/users/Breee/projects/2). Milestones are not used.

## Issue types (one `type:` label each, template in `.github/ISSUE_TEMPLATE/`)

| Type | Use when | Must contain |
|---|---|---|
| `type:feature` | new capability | what/why, links to spec/ADR/`Z-*` requirement |
| `type:enhancement` | change existing behaviour | current vs wanted |
| `type:bug` | zero behaves wrong | repro, actual vs expected, versions |
| `type:security` | way the agent can do damage / weakened invariant | attack path, severity, handling: mitigate / accept (ADR) / defer (phase) |
| `type:caveat` | known, documented limitation (`ai-docs/caveats.md` C-number) | symptom, versions, severity |
| `type:upstream` | bug/limit in a dependency (hard rule: no workarounds) | upstream link, draft `ai-docs/upstream/*.md`, caveat C-number |

Every issue body has **GOOD** (quality bar), **DONE** (checkable criteria), **PROOF plan** (commands that prove DONE). No GOOD/DONE → not ready, Status stays Backlog.

Title: imperative, specific (`zero ls shows orphan tokens`), no ticket prefixes except `upstream:`.

## Project fields

| Field | Values | Rule |
|---|---|---|
| Status | Backlog → Todo → In Progress → Review → Done | exactly one item In Progress per agent |
| Phase | 1–5 (`ai-docs/phases.md`) | always set |
| Sprint | iteration (2 weeks) | set when pulled into a sprint; unset = backlog |
| Severity | high / med / low | required for security, caveat, bug |

Blocked: label `status:blocked` + comment with blocker and link.

## Workflow

1. **Create** with `gh issue create -R Breee/z3r0 --template <type>` or body following the template; `gh project item-add 2 --owner Breee --url <issue>`; set Phase (+ Severity).
2. **Start**: Sprint set, Status In Progress.
3. **Commit**: `<what> (#N)`, small steps.
4. **Review**: Status Review. Spawn a subagent with the issue body + diff/commits + proof; it checks every DONE item and GOOD. Fix findings.
5. **Close**: comment with
   ```
   PROOF
   commits: abc123, def456
   $ <command>
   <decisive output>
   REVIEW
   subagent: <findings> → <resolution>
   ```
   then `gh issue close`, Status Done.
6. **Change of plan** (postpone, not planned, moved phase): update field/label + comment with reason immediately. Decisions → ADR in `docs/adr/`.

## Useful commands

```
gh project item-list 2 --owner Breee --format json -L 300
gh project field-list 2 --owner Breee --format json
gh issue list -R Breee/z3r0 -l type:security
```
Field values are set via `gh project item-edit --id <item> --project-id <PVT_…> --field-id <…> --single-select-option-id <…>` (or `--iteration-id`).
