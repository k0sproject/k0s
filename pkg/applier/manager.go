package applier

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	Bundles map[string]Applier
}

func (m *Manager) Run() error {
	log := logrus.WithField("component", "applier-manager")
	bundlePath := filepath.Join(constant.DataDir, "manifests")
	err := os.MkdirAll(bundlePath, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", bundlePath)
	}
	m.Bundles = make(map[string]Applier)

	go func() {
		for {
			dirs, err := util.GetAllDirs(bundlePath)
			if err != nil {
				log.Warnf("failed to read bundle dirs: %s", err)
			} else {
				for _, d := range dirs {
					if _, exists := m.Bundles[d]; !exists {
						applier, err := NewApplier(filepath.Join(bundlePath, d))
						if err != nil {
							log.Warnf("failed to create applier for %s: %w", d, err)
							continue
						}
						err = applier.Run()
						if err != nil {
							log.Warnf("failed to run applier for %s: %w", d, err)
							continue
						}
						m.Bundles[d] = applier
					} else {
						log.Debugf("applier already running for %s", d)
					}
				}
			}

			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

func (m *Manager) Stop() error {
	for _, a := range m.Bundles {
		err := a.Stop()
		if err != nil {
			logrus.Warnf("failed to stop bundle applier %s", a.Name)
		}
	}
	return nil
}
