# uniget CLI: setuid bit from the tool image is dropped on install

Target: issue on gitlab.com/uniget-org/cli (or GitHub mirror uniget-org/cli). Caveat C54 (#121), tracking #122. No existing issue found (search "setuid", "newuidmap" in uniget-org/cli and uniget-org/tools, 2026-11-18).

## What
`uniget install uidmap` (uniget v0.30.1; same code in v0.30.4, `0034ef8`) installs `newuidmap`/`newgidmap` without the setuid bit and prints:

```
WARNING  Setuid bit cannot be set for usr/local/bin/newgidmap
WARNING  Setuid bit cannot be set for usr/local/bin/newuidmap
```

The tool image has the bit (`tools/uidmap/Dockerfile.template` runs `chmod u+s`; `uniget inspect uidmap` shows `-rwsr-xr-x`). Without it, rootless podman cannot map subuids, so `uniget install podman uidmap` does not give a working rootless podman.

## Why
`pkg/archive/unpack.go`, `ExtractFileFromTar`:

```go
mode := os.FileMode(header.Mode)
...
err = outFile.Chmod(mode)
```

`tar.Header.Mode` holds Unix bits (`04000` = setuid). `os.FileMode` keeps setuid in `os.ModeSetuid` (`1<<23`), not in `04000`. `os.FileMode(04755)` is `-rwxr-xr-x`, and `Chmod` clears the bit. The warning (added in `3152b15`, 2024-03-04) reports this instead of fixing it.

Repro in Go:

```go
h := &tar.Header{Mode: 0o4755}
os.FileMode(h.Mode)       // -rwxr-xr-x  (setuid lost)
h.FileInfo().Mode()       // urwxr-xr-x  (setuid kept)
```

## Fix
Use `header.FileInfo().Mode()` (or `fs.FileMode(header.Mode&0777)` plus `os.ModeSetuid`/`ModeSetgid`/`ModeSticky` mapped from `04000`/`02000`/`01000`). Optionally keep a notice when installing setuid binaries, or only set the bit when running as root.

## Versions
uniget v0.30.1 (installed), v0.30.4 (source checked); Ubuntu 24.04; uidmap 4.11.1.
