package zero

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/policy"

	"charm.land/huh/v2"
)

// firstRun guides a new workspace whose directories have no policy file:
// hosts, permissions, then the file. It asks only at a terminal; the answer
// yes approves the workspace policy shown, and nothing beyond it (ADR 0058).
func (s *Sandbox) firstRun() error {
	if s.Approve || !core.IsTerminal(os.Stdin) || !core.IsTerminal(os.Stderr) {
		return nil
	}
	dirs, found, names := policy.FirstRunPlan(s.repos())
	if len(dirs) == 0 {
		return nil
	}
	chosen := slices.Clone(names)
	caps := slices.Clone(policy.DefaultCaps)
	// The height is set by hand: without it huh hides options (caveat C63).
	var groups []*huh.Group
	if len(names) > 0 {
		var opts []huh.Option[string]
		for _, p := range policy.Presets {
			if slices.Contains(names, p.Name) {
				opts = append(opts, huh.NewOption(policy.PresetLabel(p), p.Name).Selected(true))
			}
		}
		groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
			Title(policy.HostsTitle).Description(policy.HostsText).
			Options(opts...).Height(2*len(opts)+5).Value(&chosen)))
	}
	var opts []huh.Option[string]
	for _, c := range policy.DefaultCaps {
		opts = append(opts, huh.NewOption(policy.CapLabel(c), c).Selected(true))
	}
	for _, c := range policy.OptionalCaps {
		opts = append(opts, huh.NewOption(policy.CapLabel(c), c))
	}
	groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
		Title(policy.CapsTitle).Description(policy.CapsText).
		Options(opts...).Height(2*len(opts)+4).Value(&caps).Validate(policy.CheckChosenCaps)))
	fmt.Fprintf(os.Stderr, "first run: %s has no .zero/policy.yaml yet\n", strings.Join(dirs, ", "))
	if err := huh.NewForm(groups...).WithOutput(os.Stderr).Run(); err != nil {
		return firstRunEnd(err)
	}
	files, shown, err := policy.FirstRunFiles(s.repos(), dirs, found, chosen, caps)
	if err != nil {
		return err
	}
	for _, top := range dirs {
		fmt.Fprintf(os.Stderr, "\n%s:\n%s", filepath.Join(top, ".zero", "policy.yaml"), files[top])
	}
	if len(s.repos()) > 1 {
		fmt.Fprintf(os.Stderr, "\nthe workspace policy (every directory together):\n%s", policy.Render(shown))
	}
	ok := true
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title("Write and use this policy?").
		Description("Enter: yes. No: nothing is written, the default applies.").
		Value(&ok))).WithOutput(os.Stderr).Run(); err != nil {
		return firstRunEnd(err)
	}
	if !ok {
		return nil
	}
	if err := policy.WriteFirstRun(dirs, files); err != nil {
		return err
	}
	s.shown = shown
	return nil
}

// firstRunEnd: leaving the questions (esc, ctrl+c) stops the command.
func firstRunEnd(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return errors.New("stopped; nothing written")
	}
	return err
}
