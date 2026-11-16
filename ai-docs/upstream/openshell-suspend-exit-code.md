# openshell sandbox suspend/resume: exit code 0 when the driver refuses

Project: NVIDIA/OpenShell — version 0.1.8 (podman and vm drivers). Caveat C50. No existing issue found (search "suspend exit code", 2026-11-16).

## What
`openshell sandbox suspend <name>` and `resume <name>` print an error but exit 0.

## Repro
```
openshell sandbox create --name t --detach
openshell sandbox suspend t; echo "exit=$?"
```
Actual: `message: "compute driver does not support sandbox suspend"`, `exit=0`, sandbox stays Ready.
Expected: non-zero exit code.

## Why it matters
Scripts cannot tell a refused suspend from a successful one.

## Also
Neither shipped driver supports suspend/resume; the help text does not say which does.
