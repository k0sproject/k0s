package component

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/Mirantis/mke/pkg/etcd"
)

// Manager manages components
type Manager struct {
	components []Component
}

// NewManager creates a manager
func NewManager() *Manager {
	return &Manager{
		components: []Component{},
	}
}

// Add adds a component to the manager
func (m *Manager) Add(component Component) {
	m.components = append(m.components, component)
}

// Init initializes all managed components
func (m *Manager) Init() error {
	g := new(errgroup.Group)

	for _, comp := range m.components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("initializing %v\n", compName)
		c := comp
		g.Go(c.Init)
	}
	err := g.Wait()
	return err
}

// Start starts all managed components
func (m *Manager) Start() error {
	for _, comp := range m.components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("starting %v", compName)
		if compName == "APIServer" {
			if componentIsReady("kube-apiserver", "etcd") {
				if err := comp.Run(); err != nil {
					return err
				}
			}
		} else {
			// start services component without health-check
			if err := comp.Run(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Stop stops all managed components
func (m *Manager) Stop() error {
	var ret error = nil
	for i := len(m.components) - 1; i >= 0; i-- {
		if err := m.components[i].Stop(); err != nil {
			logrus.Errorf("failed to stop component: %s", err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to stop components")
			}
		}
	}
	return ret
}

// componentIsReady runs a loop to wait for a specific service to become healthy
func componentIsReady(comp string, dep string) bool {
	log := logrus.WithField("component", comp)
	ctx := context.Background()

	log.Infof("waiting for %v health", dep)

	// run the health check
	update := func(quit chan bool) {
		go func() {
			for {
				log.Infof("checking %v health", dep)
				err := getHealthCheck(dep)
				if err != nil {
					log.Errorf("health-check: %v might be down: %v", dep, err)
				} else {
					close(quit)
				}
				select {
				case <-quit:
					return
				default:
				}
			}
		}()
	}

	quit := make(chan bool)

	// loop forever, until the context is canceled, or until the component is healthy
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			update(quit)
		case <-quit:
			return true
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Minute):
			log.Errorf("timeout waiting for %v health-check", dep)
			return false
		}
	}
}

func getHealthCheck(serviceName string) error {
	var err error
	switch serviceName {
	case "etcd":
		err = etcd.CheckEtcdReady()
	default:
		return fmt.Errorf("no corresponding health-check found. leaving %s in unhealthy state", serviceName)
	}
	return err
}
