package zero

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed install.sh
var installScript string

// Install sets up the host (dedicated gateway user, pinned OpenShell, rootless
// podman) for the user who invoked sudo. It must run as root.
func Install() error {
	lu := os.Getenv("SUDO_USER")
	if os.Geteuid() != 0 || lu == "" || lu == "root" {
		return fmt.Errorf("run as: sudo zero install")
	}
	cmd := exec.Command("sh", "-s", lu)
	cmd.Stdin = strings.NewReader(installScript)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
