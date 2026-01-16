// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mw "github.com/k0sproject/k0s/internal/pkg/middleware"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

// StaticPods represents the way how k0s manages node-local static pod manifests
// exposed to the kubelet.
type StaticPods interface {
	// ManifestURL returns the HTTP URL that can be used by the kubelet to
	// obtain static pod manifests managed by this StaticPods instance.
	ManifestURL() (*url.URL, error)

	// ClaimStaticPod returns a new, empty StaticPod associated with the given
	// namespace and name. Note that only one StaticPod for a given combination
	// may be claimed, and needs to be dropped when no longer in use.
	ClaimStaticPod(namespace, name string) (StaticPod, error)
}

// StaticPod represents a single, node-local static pod manifest exposed to the
// kubelet, managed by k0s.
type StaticPod interface {
	// SetManifest replaces the manifest for this static pod. The new manifest
	// has to be a valid pod manifest, and needs to have the same namespace and
	// name that have been used when claiming this pod.
	SetManifest(podResource any) error

	// Clear removes this static pod manifest from kubelet, leaving it claimed.
	// A new manifest can be set via SetManifest.
	Clear()

	// Drop drops this static pod, removing it from the kubelet and invalidating
	// this instance. When Drop returns, subsequent calls to SetManifest will
	// err out and the pod can be reclaimed.
	Drop()
}

// staticPodID uniquely identifies static pod manifests managed by staticPods.
type staticPodID struct {
	namespace, name string
}

// staticPod implements the StaticPod interface.
type staticPod struct {
	staticPodID // initially set, immutable

	mu           sync.Mutex
	manifestPtr  atomic.Value // Store only when mu is locked, concurrent Load is okay
	update, drop func()
}

// staticPods implements the StaticPods interface, as well as the Component
// interface, so that it can be hooked in as a k0s component.
type staticPods struct {
	log logrus.FieldLogger // initially set, immutable

	mu        sync.Mutex
	lifecycle staticPodsLifecycle

	contentPtr  atomic.Value // Store only when mu is locked, concurrent Load is okay
	claimedPods map[staticPodID]*staticPod

	hostAddr    string // guaranteed to be initialized when started, immutable afterwards
	closeServer func() error
}

var _ manager.Ready = (*staticPods)(nil)

// NewStaticPods creates a new static_pods component.
func NewStaticPods() interface {
	StaticPods
	manager.Component
} {
	staticPods := &staticPods{
		log:         logrus.WithFields(logrus.Fields{"component": "static_pods"}),
		claimedPods: make(map[staticPodID]*staticPod),
	}
	staticPods.contentPtr.Store(generateContent(nil))
	return staticPods
}

func (s *staticPods) ManifestURL() (*url.URL, error) {
	if lifecycle := s.peekLifecycle(); lifecycle < staticPodsStarted {
		return nil, staticPodsNotYet(staticPodsStarted)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return &url.URL{Scheme: "http", Host: s.hostAddr, Path: "/manifests"}, nil
}

func (s *staticPods) ClaimStaticPod(namespace, name string) (StaticPod, error) {
	staticPod, err := newStaticPod(namespace, name)
	if err != nil {
		return nil, err
	}

	id := staticPod.staticPodID

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.claimedPods[id]; ok {
		return nil, fmt.Errorf("%s is already claimed", &id)
	}

	// hook the static pod into this instance
	staticPod.drop = func() { s.drop(id) }
	staticPod.update = s.update
	s.claimedPods[id] = staticPod

	return staticPod, nil
}

func newStaticPod(namespace, name string) (*staticPod, error) {
	if errs := validation.IsDNS1123Label(namespace); errs != nil {
		return nil, fmt.Errorf("invalid namespace: %q: %s", namespace, strings.Join(errs, ", "))
	}
	if errs := validation.IsDNS1123Subdomain(name); errs != nil {
		return nil, fmt.Errorf("invalid name: %q: %s", name, strings.Join(errs, ", "))
	}

	staticPod := staticPod{staticPodID: staticPodID{namespace, name}}
	staticPod.manifestPtr.Store([]byte{})

	return &staticPod, nil
}

func (p *staticPod) SetManifest(podResource any) error {
	// convert podResource into JSON
	var jsonBytes []byte
	var err error
	switch data := podResource.(type) {
	case []byte:
		jsonBytes, err = yaml.YAMLToJSON(data)
		if err != nil {
			return err
		}
	case string:
		jsonBytes, err = yaml.YAMLToJSON([]byte(data))
		if err != nil {
			return err
		}
	default:
		jsonBytes, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}

	if err := validatePodResource(&p.staticPodID, jsonBytes); err != nil {
		return err
	}

	// Update this pod's content, if not already dropped.
	p.mu.Lock()
	update := p.update
	if update == nil {
		p.mu.Unlock()
		return errors.New("already dropped")
	}
	p.manifestPtr.Store(jsonBytes)
	p.mu.Unlock()

	// Update the content of the enclosing staticPods instance, without holding
	// this pod's lock, so that there's no potential deadlocks. The update
	// method itself will check if the staticPods instance has been stopped
	// concurrently, anyways.
	update()
	return nil
}

func validatePodResource(claimedID *staticPodID, json []byte) error {
	// Validate the manifest to have only fields that are valid for pods.
	var pod corev1.Pod
	err := yaml.UnmarshalStrict(json, &pod)
	if err != nil {
		return err
	}

	// Validate that it's actually a pod.
	if pod.APIVersion != "v1" || pod.Kind != "Pod" {
		return fmt.Errorf("not a Pod: %s/%s", pod.APIVersion, pod.Kind)
	}

	// Validate that the pod is matching this claim.
	if actualID := (staticPodID{pod.Namespace, pod.Name}); actualID != *claimedID {
		return fmt.Errorf("attempt to set the manifest to %q, whereas %q was claimed", &actualID, claimedID)
	}

	return nil
}

func (p *staticPod) Clear() {
	// Clear this pod's content.
	p.mu.Lock()
	update := p.update
	p.manifestPtr.Store([]byte{})
	p.mu.Unlock()

	// If this pod hasn't been dropped already, update the content of the
	// enclosing staticPods instance. Do this without holding this pod's lock,
	// so that there's no potential deadlocks. The update method itself will
	// check if the staticPods instance has been stopped concurrently, anyways.
	if update != nil {
		update()
	}
}

func (p *staticPod) Drop() {
	// Clear this pod's content, and unhook it from its enclosing staticPods instance.
	p.mu.Lock()
	drop := p.drop
	p.update = nil
	p.drop = nil
	p.manifestPtr.Store([]byte{})
	p.mu.Unlock()

	// If this pod hasn't been dropped already, drop it from the enclosing
	// staticPods instance. Do this without holding this pod's lock, so that
	// there's no potential deadlocks. The drop method will check if the
	// staticPods instance has been stopped concurrently, anyways.
	if drop != nil {
		drop()
	}
}

// String returns a loggable representation for staticPodIds.
func (i *staticPodID) String() string {
	return fmt.Sprintf("%s/%s", i.namespace, i.name)
}

// update regenerates the content and stores it.
func (s *staticPods) update() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Don't update anything if this instance is not started anymore.
	if s.peekLifecycle() > staticPodsStarted {
		return
	}

	s.contentPtr.Store(generateContent(s.claimedPods))
}

// drop removes the given id, regenerates the content and stores it.
func (s *staticPods) drop(id staticPodID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// No need to drop anything if this instance is not started anymore.
	if s.peekLifecycle() > staticPodsStarted {
		return
	}

	delete(s.claimedPods, id)
	s.contentPtr.Store(generateContent(s.claimedPods))
}

// generateContent returns a JSON encoded list of pods, to be consumed by the kubelet.
func generateContent(pods map[staticPodID]*staticPod) []byte {
	var buf bytes.Buffer

	buf.WriteString(`{"apiVersion":"v1","kind":"PodList","items":[`)

	var needsComma bool
	for _, pod := range pods {
		manifest := pod.loadManifest()
		if len(manifest) > 0 {
			if needsComma {
				buf.WriteRune(',')
			} else {
				needsComma = true
			}
			buf.Write(manifest)
		}
	}

	buf.WriteString("]}")

	return buf.Bytes()
}

func (p *staticPod) loadManifest() []byte {
	return p.manifestPtr.Load().([]byte)
}

func (s *staticPods) content() []byte {
	return s.contentPtr.Load().([]byte)
}

func (s *staticPods) Init(context.Context) error {
	// Nothing to initialize, but still check if this component is used correctly.
	if !s.transition(staticPodsUninitialized, staticPodsInitialized) {
		return staticPodsAlready(s.peekLifecycle())
	}

	return nil
}

func (s *staticPods) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.transition(staticPodsInitialized, staticPodsStarting) {
		lifecycle := s.peekLifecycle()
		if lifecycle < staticPodsInitialized {
			return staticPodsNotYet(staticPodsInitialized)
		}

		return staticPodsAlready(lifecycle)
	}

	// Open a TCP port to listen for incoming HTTP requests.
	listener, err := net.Listen("tcp", "127.0.0.1:") // FIXME: Support IPv6 / dual-stack?
	if err != nil {
		s.transition(staticPodsStarting, staticPodsInitialized)
		return err
	}

	// Initialize a new HTTP server for static pods.
	addr := listener.Addr().String()
	log := s.log.WithField("local_addr", addr)
	srv := newStaticPodsServer(log, s.content)
	srv.Addr = addr

	// Fire up the goroutine to accept HTTP connections.
	log.Info("Serving HTTP requests")
	var serveErr error
	stopServing, doneServing := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(doneServing)

		retryAfter := time.After(10 * time.Second)
		err := srv.Serve(listener)

		for {
			select {
			case <-stopServing:
				if !errors.Is(err, http.ErrServerClosed) {
					serveErr = fmt.Errorf("unexpected HTTP server error: %w", err)
				}
				return
			default:
			}

			log.WithError(err).Error("HTTP server terminated unexpectedly")

			select {
			case <-retryAfter:
				retryAfter = time.After(10 * time.Second)
				err = srv.ListenAndServe()
			case <-stopServing:
				serveErr = fmt.Errorf("HTTP server terminated unexpectedly: %w", err)
				return
			}
		}
	}()

	// Store the handles.
	s.hostAddr = addr
	s.closeServer = func() error {
		select {
		case <-stopServing:
		default:
			close(stopServing)
		}

		err := srv.Close()
		if err == nil {
			log.Info("HTTP server closed")
			<-doneServing
			return serveErr
		}

		err = fmt.Errorf("while closing HTTP server: %w", err)
		select {
		case <-doneServing:
			if serveErr != nil {
				return errors.Join(serveErr, err)
			}
		default:
		}

		return err
	}

	// This instance started successfully, everything is setup and running.
	s.transition(staticPodsStarting, staticPodsStarted)
	return nil
}

func newStaticPodsServer(log logrus.FieldLogger, contentFn func() []byte) *http.Server {
	mux := http.NewServeMux()

	// The main endpoint to be consumed by the kubelet.
	mux.Handle("/manifests", mw.AllowMethods(http.MethodGet)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithField("remote_addr", r.RemoteAddr)
			content := contentFn()
			log.Debugf("Writing content: %s", string(content))
			if _, err := w.Write(content); err != nil {
				log.WithError(err).Warn("Failed to write HTTP response")
			}
		})))

	// Internal health check.
	mux.Handle("/manifests/_healthz", mw.AllowMethods(http.MethodGet)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithField("remote_addr", r.RemoteAddr)
			log.Debugf("Answering health check")
			w.WriteHeader(http.StatusNoContent)
		})))

	return &http.Server{
		Handler:      mux,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
}

func (s *staticPods) Stop() error {
	s.mu.Lock()

	if !s.transition(staticPodsStarted, staticPodsStopping) {
		lifecycle := s.peekLifecycle()
		if lifecycle < staticPodsStarted {
			s.mu.Unlock()
			return staticPodsNotYet(staticPodsStarted)
		}
	}

	// Swap out all the claimed pods.
	claimedPods := s.claimedPods
	s.claimedPods = map[staticPodID]*staticPod{}

	// Fire up a goroutine for every claimed pod that drops
	// it concurrently, so that there's no deadlocks.
	var wg sync.WaitGroup
	for _, pod := range claimedPods {
		wg.Go(func() {
			pod.mu.Lock()
			defer pod.mu.Unlock()
			pod.update = nil
			pod.drop = nil
			pod.manifestPtr.Store([]byte{})
		})
	}

	s.mu.Unlock()

	err := s.closeServer()
	wg.Wait()
	s.contentPtr.Store([]byte{})

	s.transition(staticPodsStopping, staticPodsStopped)
	return err
}

// Health-check interface
func (s *staticPods) Ready() error {
	url, err := s.ManifestURL()
	if err != nil {
		return err
	}

	// Create a one-shot HTTP client.
	client := http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	defer client.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.JoinPath("_healthz").String(), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected HTTP response status: %s", resp.Status)
	}
	return nil
}

type staticPodsLifecycle int32

const (
	staticPodsUninitialized = staticPodsLifecycle(iota)
	staticPodsInitialized
	staticPodsStarting
	staticPodsStarted
	staticPodsStopping
	staticPodsStopped
)

func (l staticPodsLifecycle) String() string {
	switch l {
	case staticPodsUninitialized, staticPodsInitialized, staticPodsStarting:
		return "initialized"
	case staticPodsStarted:
		return "running"
	case staticPodsStopping:
		return "stopping"
	case staticPodsStopped:
		return "stopped"
	default:
		return fmt.Sprintf("<unknown (%d)>", l)
	}
}

func (s *staticPods) transition(old, new staticPodsLifecycle) bool {
	return atomic.CompareAndSwapInt32((*int32)(&s.lifecycle), int32(old), int32(new))
}

func (s *staticPods) peekLifecycle() staticPodsLifecycle {
	return staticPodsLifecycle(atomic.LoadInt32((*int32)(&s.lifecycle)))
}

type staticPodsNotYet staticPodsLifecycle

func (l staticPodsNotYet) Error() string {
	return fmt.Sprintf("static_pods component is not yet %s", staticPodsLifecycle(l))
}

type staticPodsAlready staticPodsLifecycle

func (l staticPodsAlready) Error() string {
	return fmt.Sprintf("static_pods component is already %s", staticPodsLifecycle(l))
}
