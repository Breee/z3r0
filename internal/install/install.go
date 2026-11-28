package install

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed install.sh
var installScript string

// Install sets up the host (dedicated gateway user, pinned OpenShell, the
// container runtime) for the user who invoked sudo. It must run as root.
// runtime is podman, docker-rootless or docker (ADR 0057); empty keeps the
// installed one.
func Install(runtime string) error {
	lu := os.Getenv("SUDO_USER")
	if os.Geteuid() != 0 || lu == "" || lu == "root" {
		return fmt.Errorf("run as: sudo zero install")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("sh", "-s", lu, exe, runtime)
	cmd.Stdin = strings.NewReader(installScript)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
