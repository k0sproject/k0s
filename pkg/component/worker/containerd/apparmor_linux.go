// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/k0sproject/k0s/internal/os/linux/apparmor"
	"github.com/k0sproject/k0s/pkg/supervisor"

	containerdapparmor "github.com/containerd/containerd/v2/contrib/apparmor"
	"github.com/sirupsen/logrus"
)

const (
	dumpContainerAppArmorDefaultProfileHookMarker = "__K0S_CONTAINERD_DUMP_CONTAINER_APPARMOR_DEFAULT_PROFILE"

	// https://github.com/containerd/containerd/blob/v2.3.6/internal/cri/sputil/securityprofile_linux.go#L34
	containerAppArmorDefaultProfileName = "cri-containerd.apparmor.d"
)

func DumpContainerAppArmorDefaultProfileHook() {
	if len(os.Args) != 2 || os.Args[1] != dumpContainerAppArmorDefaultProfileHookMarker {
		return
	}

	profile, err := containerdapparmor.DumpDefaultProfile(containerAppArmorDefaultProfileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: failed to dump profile:", err)
		os.Exit(1)
	}
	_, err = os.Stdout.WriteString(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: failed to write to standard output:", err)
		os.Exit(1)
	}

	os.Exit(0)
}

type AppArmorConfig struct {
	ProfileManagement AppArmorProfileManagement
	ProfileName       string
}

type AppArmorProfileManagement uint8

const (
	NoAppArmorProfileManagement AppArmorProfileManagement = iota
	AppArmorProfilesManagedAutomatically
	AppArmorProfilesManagedOnHost
)

// Sets up the AppArmor profiles for containerd according to the configuration
// and returns the starter that confines the containerd process, if any.
func (c *Component) setupAppArmor(ctx context.Context) (supervisor.ProcessStarter, error) {
	log := logrus.WithField("component", "containerd")

	switch cfg := &c.cfg.AppArmor; cfg.ProfileManagement {
	case AppArmorProfilesManagedAutomatically:
		if err := apparmor.ValidateProfileName(cfg.ProfileName); err != nil {
			return nil, fmt.Errorf("invalid profile name %s: %w", cfg.ProfileName, err)
		}
		return setupAppArmorAutomatically(ctx, log, cfg.ProfileName), nil

	case AppArmorProfilesManagedOnHost:
		return setupHostManagedAppArmor(log, cfg.ProfileName)

	case NoAppArmorProfileManagement:
		if status, err := apparmor.Status(); err == nil && status == apparmor.Active {
			log.Info("Neither touching nor using any AppArmor profiles")
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported AppArmor profile management setting: %d", cfg.ProfileManagement)
	}
}

func setupAppArmorAutomatically(ctx context.Context, log logrus.FieldLogger, profileName string) supervisor.ProcessStarter {
	if status, err := apparmor.Status(); err != nil {
		log.WithError(err).Warn("Failed to determine AppArmor status, disabling profile management")
		return nil
	} else if status != apparmor.Active {
		log.Debug("AppArmor is not active, disabling profile management")
		return nil
	}
	if os.Getenv("container") != "" {
		// containerd won't use AppArmor for containers in there, either.
		log.Debug("Running inside a container, disabling AppArmor profile management")
		return nil
	}

	// Confining containerd changes the label of runc and of all the containers
	// without a profile of their own. Profiles that refer to them by label
	// won't match anymore. Only enable that on hosts that need it.
	if restricted, _, err := apparmor.UnprivilegedUnconfinedRestricted(); err != nil {
		log.WithError(err).Warn("Failed to determine if AppArmor restricts unprivileged unconfined tasks, disabling profile management")
		return nil
	} else if !restricted {
		log.Debug("AppArmor doesn't restrict unprivileged unconfined tasks, disabling profile management")
		return nil
	}

	// The generated profile permits everything that "unconfined" permits, but
	// it isn't the special "unconfined" profile, so the kernel doesn't stack
	// transitions out of it even when it restricts unprivileged unconfined
	// tasks.
	profile, err := apparmor.ReplaceProfile(ctx, profileName, strings.NewReader(
		"abi <abi/4.0>, profile "+profileName+" flags=(unconfined) { userns, }",
	))
	if err != nil {
		// Replacing AppArmor profiles is unsupported on this machine.
		if errors.Is(err, errors.ErrUnsupported) {
			log.WithError(err).Info("Not transitioning containerd into an AppArmor profile")
			return nil
		}

		// Parsers older than 4.0 know neither that ABI nor the userns rule.
		log.WithError(err).Debug("Failed to load AppArmor profile with policy ABI 4.0, retrying without")
		if profile, err = apparmor.ReplaceProfile(ctx, profileName, strings.NewReader(
			"profile "+profileName+" flags=(unconfined) { }",
		)); err != nil {
			log.WithError(err).
				WithField("profileName", profileName).
				Warn("Failed to load AppArmor profile for containerd, running without transitioning it")
			return nil
		}
	}

	log.WithField("profileName", profileName).Info("Loaded AppArmor profile for containerd")

	// The generated profile is a compatibility mitigation, not a security
	// enforcement. Start containerd unconfined whenever the confinement can't
	// be requested. That's preferable to a containerd that won't start at all.
	starter := processStarterFunc(func(cmd *exec.Cmd) error {
		err := profile.Start(cmd)
		if errors.Is(err, apparmor.ErrExecTransitionRequest) {
			log.WithError(err).
				WithField("profileName", profileName).
				Warn("Starting containerd without transitioning it into an AppArmor profile")
			return cmd.Start()
		}
		return err
	})

	// containerd bakes its own label into its default container profile and
	// loads it, but won't replace an already loaded one. Regenerate it here
	// using the currently configured profile so that a changed profile name
	// is reflected in the default container profile as well.
	if text, err := dumpContainerAppArmorDefaultProfile(ctx, profile); err != nil {
		log.WithError(err).
			WithField("profileName", containerAppArmorDefaultProfileName).
			Warn("Failed to dump container AppArmor default profile, starting containerd without transitioning it into an AppArmor profile")
		return nil
	} else if _, err = apparmor.ReplaceProfile(ctx, containerAppArmorDefaultProfileName, bytes.NewReader(text)); err != nil {
		log.WithError(err).
			WithField("profileName", containerAppArmorDefaultProfileName).
			Warn("Failed to replace container AppArmor default profile, starting containerd without transitioning it into an AppArmor profile")
		return nil
	}

	log.WithField("profileName", containerAppArmorDefaultProfileName).Info("Loaded container AppArmor default profile")

	return starter
}

func setupHostManagedAppArmor(log logrus.FieldLogger, profileName string) (supervisor.ProcessStarter, error) {
	if status, err := apparmor.Status(); err != nil {
		return nil, fmt.Errorf("failed to determine AppArmor status: %w", err)
	} else if status != apparmor.Active {
		return nil, errors.New("AppArmor is not active")
	}

	profile, err := apparmor.LoadedProfile(profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain profile %s: %w", profileName, err)
	}

	log.WithField("profileName", profileName).Info("Confining containerd by host-provided AppArmor profile")
	return profile, nil
}

type processStarterFunc func(cmd *exec.Cmd) error

func (f processStarterFunc) Start(cmd *exec.Cmd) error { return f(cmd) }

// Dumps containerd's AppArmor default profile for containers, as containerd
// would generate it when running confined by the given profile.
func dumpContainerAppArmorDefaultProfile(ctx context.Context, profile *apparmor.Profile) ([]byte, error) {
	k0s, err := os.Executable()
	if err != nil {
		return nil, err
	}

	var defaultContainerProfile, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, k0s, dumpContainerAppArmorDefaultProfileHookMarker)
	cmd.Stdout, cmd.Stderr = &defaultContainerProfile, &stderr
	if err := profile.Start(cmd); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		if msg := bytes.TrimSpace(stderr.Bytes()); len(msg) > 0 {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}

	return defaultContainerProfile.Bytes(), nil
}
