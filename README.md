# zero

zero runs a coding agent in a sandbox on your machine. The agent works on a copy of your repositories with a short-lived GitLab token for exactly these projects. Its only way out is `git push` to the branch `zero/<workspace>`, through a gate that refuses every other branch and starts no pipeline unless your policy allows it.

## Quick start

Needs Linux on x86_64 with systemd, `podman`, `go`, `make`, `glab` (logged in) and `opencode` (connected to GitHub Copilot).

```sh
git clone https://github.com/Breee/z3r0 && cd z3r0
make install   # as your user, not with sudo: builds bin/zero and bin/zero-cp, then runs sudo bin/zero install
zero doctor

cd ~/src/your-gitlab-project
zero opencode
```

## Documentation

| | |
|---|---|
| User guide | [getting started](docs/content/docs/getting-started.md), [commands](docs/content/docs/commands.md), [policy](docs/content/docs/policy.md), [security](docs/content/docs/security.md) |
| Architecture | [docs/architecture](docs/architecture) |
| Decisions | [docs/adr](docs/adr/README.md) |
| Attack paths | [docs/attack-paths.md](docs/attack-paths.md) |
| Roadmap | [docs/roadmap.md](docs/roadmap.md) |

The user guide is a Hugo site with the [Hextra](https://imfing.github.io/hextra/) theme in `docs/` (`hugo.yaml`, `content/`). Preview it at http://localhost:1313/z3r0/ with:

```sh
hugo server --source docs
```

Build it with `hugo --source docs --panicOnWarning`; the output goes to `docs/public/`.

## Tests

```sh
go test ./internal/zero/
sh tests/attacks/forge.sh zero-<workspace>   # attack checks against a running sandbox
```
