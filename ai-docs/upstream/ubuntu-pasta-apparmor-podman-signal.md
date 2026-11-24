# Comment for Ubuntu bug 2154379: pasta is not allowed to receive signals from podman (AppArmor)

**Target:** comment ("affects me too") on https://bugs.launchpad.net/bugs/2154379 (related: https://bugs.launchpad.net/bugs/2174446, https://github.com/actions/runner-images/issues/15182). No new report.

---

Also seen with a podman newer than the packaged one, through the API socket with Docker Compose.

Steps (as a regular user, `systemctl --user start podman.socket`, no other container on a user network):

```
# compose.yaml: two services on the default compose network
podman compose up -d
podman compose down
```

Actual, 3 of 3 runs:

```
Container 17-litellm-backend-1 Error while Stopping
Error response from daemon: removing container <id> network: 1 error occurred:
	* rootless netns: kill network process: permission denied
exit status 1
```

The first container is removed; the last one of the network and the network stay. A second `podman compose down` removes them.

Expected: `down` removes both containers and the network.

Host: `/etc/apparmor.d/podman` and `/etc/apparmor.d/usr.bin.pasta` present, the pasta profile has no rule with `peer=podman`. The kernel log line was not read (no root).

Versions: Ubuntu 26.04 LTS, kernel 7.0.0-28-generic, podman 6.2.0 (not the Ubuntu package), passt 0.0~git20260120.386b5f5-1, apparmor 5.0.0~beta1-0ubuntu7, Docker Compose v5.3.1 as compose provider.
