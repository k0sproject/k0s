// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/k0sproject/k0s/internal/assets"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leasecounter"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
)

// The konnectivity server count to be advertised.
type KonnectivityServerCount struct {
	Count uint
	Rises uint64 // changes whenever the server count was observably raised
}

// Returns the server count to advertise. Same semantics as [value.Latest.Peek].
type KonnectivityServerCounter = func() (KonnectivityServerCount, <-chan struct{})

// Derives the konnectivity server count to be advertised from the observed
// controller leases, and distributes it to interested parties.
//
// Debounces count drops for one minute to reduce churn on the konnectivity mesh
// due to potentially intermittent dips, such as controller restarts or data
// store hiccups. All other changes are distributed without delay.
type KonnectivityServerCountDebouncer struct {
	advertised value.Latest[KonnectivityServerCount]

	mu          sync.Mutex
	ownRises    uint64
	pendingDrop *uint
	timer       *time.Timer
}

// See [KonnectivityServerCounter].
func (c *KonnectivityServerCountDebouncer) Peek() (KonnectivityServerCount, <-chan struct{}) {
	return c.advertised.Peek()
}

// Derives a new konnectivity server count from leases.
// Returns this controller's resulting konnectivity state after the update.
func (c *KonnectivityServerCountDebouncer) Update(leases leasecounter.Leases) *leasecounter.KonnectivityState {
	log := logrus.WithField("component", "konnectivity")

	// Sum up the server count rises of all controllers.
	var rises uint64
	// Consider both active and inactive leases, so that a lease flapping
	// between active and inactive doesn't change the sum and roll the agents
	// although its generation is unchanged. This also covers the own lease
	// being inactive right after startup, whose generation must still count.
	for i := range leases.All {
		if state := leases.All[i].Konnectivity; state != nil {
			// Don't use this controller's own requested-but-not-yet-recorded
			// generation (see below), so that the sum remains a pure function
			// of the cluster state. Each controller computes the same value
			// within the observation skew, ensuring that a leader change won't
			// roll the agents unnecessarily.
			rises += state.ServerCountRises
		}
	}
	var own *leasecounter.KonnectivityState
	if leases.Own != nil {
		own = leases.Own.Konnectivity
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Pick up the recorded server count rises, unless more have been counted
	// that haven't been recorded yet.
	if own != nil {
		c.ownRises = max(c.ownRises, own.ServerCountRises)
	}

	advertised, _ := c.advertised.Peek()
	next := advertised
	next.Rises = rises

	switch {
	case leases.ActiveCount < advertised.Count:
		// A drop is withheld for the debounce period.
		if c.pendingDrop == nil {
			log.WithFields(logrus.Fields{
				"advertised": advertised.Count,
				"observed":   leases.ActiveCount,
			}).Info("Observed a server count drop, withholding it for one minute")
			var timer *time.Timer
			timer = time.AfterFunc(time.Minute, func() {
				c.mu.Lock()
				defer c.mu.Unlock()
				if c.pendingDrop != nil {
					advertised, _ := c.advertised.Peek()
					log.WithFields(logrus.Fields{
						"observed":                 *c.pendingDrop,
						"observedServerCountRises": advertised.Rises,
					}).Info("Advertising a new server count")
					advertised.Count, c.pendingDrop = *c.pendingDrop, nil
					c.advertised.Set(advertised)
				}
				if c.timer == timer {
					c.timer = nil
				}
			})
			c.timer = timer
		} else if leases.ActiveCount != *c.pendingDrop {
			log.WithFields(logrus.Fields{
				"advertised": advertised.Count,
				"observed":   leases.ActiveCount,
			}).Info("Server count changed during the debounce period")
		}
		c.pendingDrop = new(leases.ActiveCount)

	case leases.ActiveCount > advertised.Count:
		// The lower count this controller advertised before ...
		advertisedCount := advertised.Count
		// ... or if none has been advertised before (== 0) ...
		if advertisedCount == 0 && own != nil {
			// ... the one its previous process recorded in the lease.
			advertisedCount = own.ServerCount
		}

		// If that count is lower than the new count ...
		if advertisedCount != 0 && advertisedCount < leases.ActiveCount {
			// ... the agents need to be rolled, so they'll pick it up.
			c.ownRises++
		}

		next.Count = leases.ActiveCount
		fallthrough // discard a drop

	default:
		if c.pendingDrop != nil {
			log.WithFields(logrus.Fields{
				"advertised": advertised.Count,
				"observed":   next.Count,
				"pending":    *c.pendingDrop,
			}).Debug("Discarding an outdated pending server count drop")
			c.pendingDrop = nil
			if c.timer != nil {
				c.timer.Stop()
				c.timer = nil
			}
		}
	}

	if next != advertised {
		log.WithFields(logrus.Fields{
			"observed":                 next.Count,
			"observedServerCountRises": next.Rises,
		}).Info("Advertising a new server count")
		c.advertised.Set(next)
	}

	return &leasecounter.KonnectivityState{
		ServerCount:      next.Count,
		ServerCountRises: c.ownRises,
	}
}

// Konnectivity implements the component interface for konnectivity server
type Konnectivity struct {
	Spec        *v1beta1.KonnectivitySpec // FIXME: This should be reconciled via dynamic config
	K0sVars     *config.CfgVars
	LogLevel    string
	ServerCount func() (uint, <-chan struct{})

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int

	log *logrus.Entry

	*prober.EventEmitter

	stop func()
}

var _ manager.Component = (*Konnectivity)(nil)
var _ manager.Ready = (*Konnectivity)(nil)
var _ prober.Healthz = (*Konnectivity)(nil)

// Init ...
func (k *Konnectivity) Init(ctx context.Context) error {
	var err error
	k.uid, err = users.LookupUID(constant.KonnectivityServerUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.KonnectivityServerUser, err)
		k.uid = users.RootUID
		k.EmitWithPayload("error getting UID for", err)
		logrus.WithError(err).Warn("Running konnectivity as root")
	}
	err = dir.Init(k.K0sVars.KonnectivitySocketDir, 0755)
	if err != nil {
		k.EmitWithPayload("failed to initialize socket directory", err)
		return fmt.Errorf("failed to initialize directory %s: %w", k.K0sVars.KonnectivitySocketDir, err)
	}

	err = os.Chown(k.K0sVars.KonnectivitySocketDir, k.uid, -1)
	if err != nil && os.Geteuid() == 0 {
		k.EmitWithPayload("failed to chown socket directory", err)
		return fmt.Errorf("failed to chown %s: %w", k.K0sVars.KonnectivitySocketDir, err)
	}

	k.log = logrus.WithFields(logrus.Fields{"component": "konnectivity"})
	if k.executablePath, err = assets.StageExecutable(k.K0sVars.BinDir, "konnectivity-server"); err != nil {
		k.EmitWithPayload("failed to stage konnectivity-server", err)
		return fmt.Errorf("failed to stage konnectivity-server binary: %w", err)

	}
	defer k.Emit("successfully initialized konnectivity component")

	return nil
}

// Run ..
func (k *Konnectivity) Start(ctx context.Context) error {
	serverCount, serverCountChanged := k.ServerCount()

	if err := k.runServer(ctx, serverCount); err != nil {
		k.EmitWithPayload("failed to start konnectivity server", err)
		return fmt.Errorf("failed to start konnectivity server: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		var retry <-chan time.Time
		for {
			select {
			case <-serverCountChanged:
				prevServerCount := serverCount
				serverCount, serverCountChanged = k.ServerCount()
				// restart only if the server count actually changed
				if serverCount == prevServerCount {
					continue
				}

			case <-retry:
				k.Emit("retrying to start konnectivity server")
				k.log.Info("Retrying to start konnectivity server")

			case <-ctx.Done():
				k.Emit("stopped konnectivity server")
				k.log.Info("stopping konnectivity server reconfig loop")
				return
			}

			retry = nil

			if err := k.runServer(ctx, serverCount); err != nil {
				k.EmitWithPayload("failed to start konnectivity server", err)
				k.log.WithError(err).Errorf("Failed to start konnectivity server")
				retry = time.After(10 * time.Second)
				continue
			}
		}
	}()

	k.stop = func() { cancel(); <-done }

	return nil
}

func (k *Konnectivity) serverArgs(count uint) []string {
	return stringmap.StringMap{
		"--uds-name":                  filepath.Join(k.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		"--cluster-cert":              filepath.Join(k.K0sVars.CertRootDir, "server.crt"),
		"--cluster-key":               filepath.Join(k.K0sVars.CertRootDir, "server.key"),
		"--kubeconfig":                k.K0sVars.KonnectivityKubeConfigPath,
		"--mode":                      "grpc",
		"--server-port":               "0",
		"--agent-port":                strconv.FormatInt(int64(k.Spec.AgentPort), 10),
		"--admin-port":                strconv.FormatInt(int64(k.Spec.AdminPort), 10),
		"--health-bind-address":       "localhost",
		"--health-port":               "8092",
		"--graceful-shutdown-timeout": "1ns", // Don't let lingering frontend connections block shutdown indefinitely.
		"--agent-namespace":           metav1.NamespaceSystem,
		"--agent-service-account":     "konnectivity-agent",
		"--authentication-audience":   "system:konnectivity-server",
		"--logtostderr":               "true",
		"--stderrthreshold":           "1",
		"--v":                         k.LogLevel,
		"--enable-profiling":          "false",
		"--delete-existing-uds-file":  "true",
		"--server-count":              strconv.FormatUint(uint64(count), 10),
		"--server-id":                 k.K0sVars.InvocationID,
		"--proxy-strategies":          "destHost,defaultRoute,default",
		"--cipher-suites":             constant.AllowedTLS12CipherSuiteNames(),
	}.ToArgs()
}

func (k *Konnectivity) runServer(ctx context.Context, count uint) error {
	// Stop supervisor
	if k.supervisor != nil {
		k.EmitWithPayload("restarting konnectivity server due to server count change",
			map[string]any{"serverCount": count})
		if err := k.supervisor.Stop(); err != nil {
			k.log.WithError(err).Error("Failed to stop executable")
		}
	}

	k.supervisor = &supervisor.Supervisor{
		Name:    "konnectivity",
		BinPath: k.executablePath,
		DataDir: k.K0sVars.DataDir,
		RunDir:  k.K0sVars.RunDir,
		Args:    k.serverArgs(count),
		UID:     k.uid,
	}

	if count == 0 {
		// Zero means that the actual server count isn't known yet. The server
		// advertises it as is, and agents treat zero as one, so an agent
		// connecting now would stop dialing after a single connection. Bind the
		// agent port on the loopback interface until the count is known, so
		// that agents can't reach this server in the meantime.
		k.log.Info("Starting konnectivity on the loopback interface until the server count is known")
		k.supervisor.Args = append(k.supervisor.Args, "--agent-bind-address=127.0.0.1")
	}

	err := k.supervisor.Supervise(ctx)
	if err != nil {
		k.supervisor = nil // not to make the next loop to try to stop it first
		return fmt.Errorf("failed to supervise konnectivity server: %w", err)
	}
	k.EmitWithPayload("started konnectivity server", map[string]any{"serverCount": count})

	return nil
}

// Ready implements manager.Ready.
func (k *Konnectivity) Ready() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()
	// This is somehow flipped: Check healthz instead of readyz.
	return k.health(ctx, "/healthz")
}

// Healthy implements prober.Healthz.
func (k *Konnectivity) Healthy() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()
	// This is somehow flipped: Check readyz instead of healthz.
	return k.health(ctx, "/readyz")
}

// Stop stops
func (k *Konnectivity) Stop() error {
	if k.stop != nil {
		logrus.Debug("stopping konnectivity component")
		k.stop()
	}
	if k.supervisor == nil {
		return nil
	}
	logrus.Debug("about to stop konnectivity supervisor")
	return k.supervisor.Stop()
}

func (k *Konnectivity) health(ctx context.Context, path string) error {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("no redirects allowed: %s", req.URL)
		},
	}

	endpoint := url.URL{Scheme: "http", Host: "localhost:8092", Path: path}
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected HTTP response status: %s", resp.Status)
	}

	return nil
}
