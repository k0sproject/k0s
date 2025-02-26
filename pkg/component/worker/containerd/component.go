/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containerd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"
	containerruntime "github.com/k0sproject/k0s/pkg/container/runtime"
	"github.com/k0sproject/k0s/pkg/debounce"
	"github.com/k0sproject/k0s/pkg/supervisor"

	"k8s.io/apimachinery/pkg/util/wait"
)

const containerdTomlHeader = `# k0s_managed=true
# This is a placeholder configuration for k0s managed containerd.
# If you wish to override the config, remove the first line and replace this file with your custom configuration.
# For reference see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
`
const confPathPosix = "/etc/k0s/containerd.toml"
const confPathWindows = "C:\\Program Files\\containerd\\config.toml"

const importsPathPosix = "/etc/k0s/containerd.d/"
const importsPathWindows = "C:\\etc\\k0s\\containerd.d\\"

// Component implements the component interface to manage containerd as a k0s component.
type Component struct {
	supervisor    supervisor.Supervisor
	LogLevel      string
	K0sVars       *config.CfgVars
	Profile       *workerconfig.Profile
	binaries      []string
	OCIBundlePath string
	confPath      string
	importsPath   string
}

func NewComponent(logLevel string, vars *config.CfgVars, profile *workerconfig.Profile) *Component {
	c := &Component{
		LogLevel: logLevel,
		K0sVars:  vars,
		Profile:  profile,
	}

	if runtime.GOOS == "windows" {
		c.binaries = []string{"containerd.exe", "containerd-shim-runhcs-v1.exe"}
		c.confPath = confPathWindows
		c.importsPath = importsPathWindows
	} else {
		c.binaries = []string{"containerd", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2", "runc"}
		c.confPath = confPathPosix
		c.importsPath = importsPathPosix
	}
	return c
}

var _ manager.Component = (*Component)(nil)

// Init extracts the needed binaries
func (c *Component) Init(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	for _, bin := range c.binaries {
		g.Go(func() error {
			err := assets.Stage(c.K0sVars.BinDir, bin)
			// Simply ignore the "running executable" problem on Windows for
			// now. Whenever there's a permission error on Windows and the
			// target file exists, log the error and continue.
			if err != nil &&
				runtime.GOOS == "windows" &&
				errors.Is(err, os.ErrPermission) &&
				file.Exists(filepath.Join(c.K0sVars.BinDir, bin)) {
				logrus.WithField("component", "containerd").WithError(err).Error("Failed to replace ", bin)
				return nil
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	if err := c.windowsInit(); err != nil {
		return fmt.Errorf("windows init failed: %w", err)
	}

	return nil
}

func (c *Component) windowsInit() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	// On windows we need always run containerd.exe as a service
	// https://kubernetes.io/docs/tasks/configure-pod-container/create-hostprocess-pod/#troubleshooting-hostprocess-containers
	command := fmt.Sprintf("if (-not (Get-Service -Name containerd -ErrorAction SilentlyContinue)) { %s\\containerd.exe --register-service}", c.K0sVars.BinDir)
	return winExecute(command)
}

// Run runs containerd.
func (c *Component) Start(ctx context.Context) error {
	log := logrus.WithField("component", "containerd")
	log.Info("Starting containerd")

	if err := c.setupConfig(); err != nil {
		return fmt.Errorf("failed to setup containerd config: %w", err)
	}

	var runtimeEndpoint *url.URL

	if runtime.GOOS == "windows" {
		if err := c.windowsStart(ctx); err != nil {
			return fmt.Errorf("failed to start windows server: %w", err)
		}

		runtimeEndpoint = &url.URL{Scheme: "npipe", Path: "//./pipe/containerd-containerd"}
	} else {
		socketPath := filepath.Join(c.K0sVars.RunDir, "containerd.sock")

		c.supervisor = supervisor.Supervisor{
			Name:    "containerd",
			BinPath: assets.BinPath("containerd", c.K0sVars.BinDir),
			RunDir:  c.K0sVars.RunDir,
			DataDir: c.K0sVars.DataDir,
			Args: []string{
				"--root=" + filepath.Join(c.K0sVars.DataDir, "containerd"),
				"--state=" + filepath.Join(c.K0sVars.RunDir, "containerd"),
				"--address=" + socketPath,
				"--log-level=" + c.LogLevel,
				"--config=" + c.confPath,
			},
		}

		if err := c.supervisor.Supervise(); err != nil {
			return err
		}

		runtimeEndpoint = &url.URL{Scheme: "unix", Path: socketPath}
	}

	go c.watchDropinConfigs(ctx)

	log.Debug("Waiting for containerd")
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: 100 * time.Millisecond, Factor: 1.2, Jitter: 0.05, Steps: 30,
	}, func(ctx context.Context) (bool, error) {
		rt := containerruntime.NewContainerRuntime(runtimeEndpoint)
		if lastErr = rt.Ping(ctx); lastErr != nil {
			log.WithError(lastErr).Debug("Failed to ping containerd")
			return false, nil
		}

		log.Debug("Successfully pinged containerd")
		return true, nil
	})

	if err != nil {
		if lastErr == nil {
			return fmt.Errorf("failed to ping containerd: %w", err)
		}
		return fmt.Errorf("failed to ping containerd: %w (%w)", err, lastErr)
	}

	return nil
}

func (c *Component) windowsStart(_ context.Context) error {
	if err := winExecute("Start-Service containerd"); err != nil {
		return fmt.Errorf("failed to start Windows Service %q: %w", "containerd", err)
	}
	return nil
}

func (c *Component) windowsStop() error {
	if err := winExecute("Stop-Service containerd"); err != nil {
		return fmt.Errorf("failed to stop Windows Service %q: %w", "containerd", err)
	}
	return nil
}

func (c *Component) setupConfig() error {
	// Check if the config file is user managed
	// If it is, we should not touch it

	k0sManaged, err := isK0sManagedConfig(c.confPath)
	if err != nil {
		return err
	}

	if !k0sManaged {
		logrus.Infof("containerd config file %s is not k0s managed, skipping config generation", c.confPath)
		return nil
	}
	if err := dir.Init(filepath.Dir(c.confPath), 0755); err != nil {
		return fmt.Errorf("can't create containerd config dir: %w", err)
	}
	if err := dir.Init(filepath.Dir(c.importsPath), 0755); err != nil {
		return fmt.Errorf("can't create containerd config imports dir: %w", err)
	}

	configurer := &configurer{
		loadPath:   filepath.Join(c.importsPath, "*.toml"),
		pauseImage: c.Profile.PauseImage.URI(),
		log:        logrus.WithField("component", "containerd"),
	}

	config, err := configurer.handleImports()
	if err != nil {
		return fmt.Errorf("can't handle imports: %w", err)
	}

	criConfigPath := filepath.Join(c.K0sVars.RunDir, "containerd-cri.toml")

	if err = file.AtomicWithTarget(criConfigPath).
		WithPermissions(0644).
		WriteString(config.CRIConfig); err != nil {
		return fmt.Errorf("can't create containerd CRI config: %w", err)
	}

	if err := file.AtomicWithTarget(c.confPath).
		WithPermissions(0644).
		Do(func(f file.AtomicWriter) error {
			w := bufio.NewWriter(f)
			if _, err := w.WriteString(containerdTomlHeader); err != nil {
				return err
			}
			if err := toml.NewEncoder(w).Encode(map[string]any{
				"version": 2,
				"imports": append(config.ImportPaths, criConfigPath),
			}); err != nil {
				return err
			}
			return w.Flush()
		}); err != nil {
		return fmt.Errorf("can't create containerd config: %w", err)
	}

	return nil
}

func (c *Component) watchDropinConfigs(ctx context.Context) {
	log := logrus.WithField("component", "containerd")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithError(err).Error("failed to create watcher for drop-ins")
		return
	}
	defer watcher.Close()

	err = watcher.Add(c.importsPath)
	if err != nil {
		log.WithError(err).Error("failed to watch for drop-ins")
		return
	}

	debouncer := debounce.Debouncer[fsnotify.Event]{
		Input:   watcher.Events,
		Timeout: 3 * time.Second,
		Filter: func(item fsnotify.Event) bool {
			switch item.Op {
			case fsnotify.Create, fsnotify.Remove, fsnotify.Write, fsnotify.Rename:
				return true
			default:
				return false
			}
		},
		Callback: func(fsnotify.Event) { c.restart() },
	}

	// Consume and log any errors from watcher
	go func() {
		for {
			err, ok := <-watcher.Errors
			if !ok {
				return
			}
			log.WithError(err).Error("error while watching drop-ins")
		}
	}()

	log.Infof("started to watch events on %s", c.importsPath)

	err = debouncer.Run(ctx)
	if err != nil {
		log.WithError(err).Warn("dropin watch bouncer exited with error")
	}
}

func (c *Component) restart() {
	log := logrus.WithFields(logrus.Fields{"component": "containerd", "phase": "restart"})

	log.Info("restart requested")
	if err := c.setupConfig(); err != nil {
		log.WithError(err).Warn("failed to resolve config")
		return
	}
	if runtime.GOOS == "windows" {

		if err := c.windowsStop(); err != nil {
			log.WithError(err).Warn("failed to stop windows service")
			return
		}
		if err := c.windowsStart(context.Background()); err != nil {
			log.WithError(err).Warn("failed to start windows service")
			return
		}
	} else {
		p := c.supervisor.GetProcess()
		if err := p.Signal(syscall.SIGHUP); err != nil {
			log.WithError(err).Warn("failed to send SIGHUP")
		}

	}
}

// Stop stops containerd.
func (c *Component) Stop() error {
	if runtime.GOOS == "windows" {
		return c.windowsStop()
	}
	c.supervisor.Stop()
	return nil
}

// This is the md5sum of the default k0s containerd config file before 1.27
const pre1_27ConfigSum = "59039b43303742a5496b13fd57f9beec"

// isK0sManagedConfig checks if the config file is k0s managed:
//   - If the config file doesn't exist, it's k0s managed.
//   - If the config file's md5sum matches the pre 1.27 config, it's k0s managed.
//   - If the config file starts with the magic marker line "# k0s_managed=true",
//     it's k0s managed.
func isK0sManagedConfig(path string) (_ bool, err error) {
	// If the file does not exist, it's k0s managed (new install)
	if !file.Exists(path) {
		return true, nil
	}
	pre1_27Managed, err := isPre1_27ManagedConfig(path)
	if err != nil {
		return false, err
	}
	if pre1_27Managed {
		return true, nil
	}
	// Check if the config file has the magic marker
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		switch scanner.Text() {
		case "": // K0s versions before 1.30 had a leading empty line.
			continue
		case "# k0s_managed=true":
			return true, nil
		}
	}
	return false, scanner.Err()
}

func isPre1_27ManagedConfig(path string) (bool, error) {
	// Check MD5 sum of the config file
	// If it matches the pre 1.27 config, it's k0s managed
	md5sum := md5.New()
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err := io.Copy(md5sum, f); err != nil {
		return false, err
	}

	sum := md5sum.Sum(nil)

	pre1_27ConfigSumBytes, err := hex.DecodeString(pre1_27ConfigSum)
	if err != nil {
		return false, err
	}

	if bytes.Equal(pre1_27ConfigSumBytes, sum) {
		return true, nil
	}

	return false, nil
}
