# containers/image: sigstoreSigned fulcio cannot match GitHub Actions (URI SAN) identities

Target: github.com/containers/container-libs (image). Existing: no upstream issue found; same problem noted in jfroy/etincelle#2.

**What:** `sigstoreSigned` with `fulcio` requires `oidcIssuer` + `subjectEmail`. Keyless certs from GitHub Actions (issuer `https://token.actions.githubusercontent.com`) have a URI SAN (`https://github.com/OWNER/REPO/.github/workflows/X.yml@refs/heads/main`), no email. Such images cannot be verified by policy.json.

**Why it matters:** keyless CI signing is the common sigstore path; podman users must fall back to long-lived keys or no verification.

**How (proposal):** add `subjectURI` / `subjectURIRegexp` (like cosign `--certificate-identity[-regexp]`) as alternative to `subjectEmail` in `fulcio`.

**Repro:** podman 6.2.0. Image `ghcr.io/breee/z3r0-dev@sha256:a279d03c…`, `cosign verify --certificate-identity-regexp '^https://github.com/Breee/z3r0/.github/workflows/dev-image.yml@' --certificate-oidc-issuer https://token.actions.githubusercontent.com` succeeds; no policy.json fulcio block can express this identity.
