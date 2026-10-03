// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// LoadProfile loads the worker profile with the given profile name from
// Kubernetes, using cacheDir as cache folder.
func LoadProfile(ctx context.Context, kubeconfig clientcmd.KubeconfigGetter, cacheDir, profileName string) (*Profile, error) {
	clientConfig, err := kubeutil.ClientConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client config: %w", err)
	}

	clientFactory := func(host string) (kubernetes.Interface, error) {
		clientConfig := *clientConfig
		if host != "" {
			clientConfig.Host = host
		}
		return kubernetes.NewForConfig(&clientConfig)
	}

	return loadProfile(ctx, logrus.StandardLogger(), clientFactory, cacheDir, profileName)
}

func loadProfile(ctx context.Context, log logrus.FieldLogger, clientFactory func(host string) (kubernetes.Interface, error), cacheDir, profileName string) (*Profile, error) {
	cachedAPIServerAddresses := loadAPIServerAddressesFromCache(log, cacheDir)
	if len(cachedAPIServerAddresses) < 1 {
		log.Info("No cached API server addresses found")
	} else {
		log.Debugf("Loaded cached API server addresses: %v", cachedAPIServerAddresses)
	}

	configMapName := configMapNameForProfile(profileName)
	cachedAPIServerAddresses = append(cachedAPIServerAddresses, "") // Always try out the unmodified kubeconfig, too.

	var configMapData map[string]string
	if err := retry.Do(
		func() (err error) {
			configMapData, err = loadConcurrently(ctx, cachedAPIServerAddresses,
				func(ctx context.Context, address string) (map[string]string, error) {
					client, err := clientFactory(address)
					if err != nil {
						return nil, err
					}

					cm, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, configMapName, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}

					return cm.Data, nil
				},
			)
			return err
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.Delay(500*time.Millisecond),
		retry.OnRetry(func(attempt uint, err error) {
			log.WithError(err).Debugf("Failed to load configuration for worker profile in attempt #%d, retrying after backoff", attempt+1)
		}),
	); err != nil {
		if apierrors.IsUnauthorized(err) {
			err = fmt.Errorf("the k0s worker node credentials are invalid, the node needs to be rejoined into the cluster with a fresh bootstrap token: %w", err)
		}

		// A status response means that the API server answered and rejected the
		// request. Only report failures to reach it at all as unreachable.
		var status apierrors.APIStatus
		if errors.As(err, &status) {
			return nil, err
		}

		return nil, &APIUnreachableError{Err: err}
	}

	profile, err := FromConfigMapData(configMapData)
	if err != nil {
		return nil, err
	}

	if err := storeInCacheDir(cacheDir, newStoredWorkerProfile(profileName, configMapData)); err != nil {
		return nil, err
	}

	return profile, nil
}

// APIUnreachableError indicates that the worker profile couldn't be retrieved
// because all attempts to reach the Kubernetes API failed without ever getting
// a response from it. It wraps the last error encountered.
type APIUnreachableError struct{ Err error }

func (e *APIUnreachableError) Error() string { return e.Err.Error() }
func (e *APIUnreachableError) Unwrap() error { return e.Err }

// IsAPIUnreachable returns true if err indicates that the worker profile
// couldn't be retrieved because the Kubernetes API couldn't be reached at all,
// as opposed to the API rejecting the request, the data it returned being
// unusable, or a problem with the local setup.
func IsAPIUnreachable(err error) bool {
	var unreachable *APIUnreachableError
	return errors.As(err, &unreachable)
}

// LoadProfileFromCache loads the worker profile with the given profile name
// from the cache folder cacheDir, i.e. the profile that has been stored there by
// the last successful call to [LoadProfile] or [WatchProfile]. It refuses to
// return profiles that have been cached for another profile name or another
// Kubernetes minor version. If there's no cached profile at all, the returned
// error will match [fs.ErrNotExist].
func LoadProfileFromCache(cacheDir, profileName string) (*Profile, error) {
	cached, err := loadFromCacheDir(cacheDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no worker profile has been cached yet in %s: %w", cacheDir, err)
		}

		return nil, err
	}

	if cached.Name != profileName {
		return nil, fmt.Errorf("cached worker profile is for profile %q, not %q", cached.Name, profileName)
	}

	if cached.KubernetesVersion == "" {
		return nil, errors.New("cached worker profile doesn't record a Kubernetes version")
	}

	if cached.KubernetesVersion != constant.KubernetesMajorMinorVersion {
		return nil, fmt.Errorf("cached worker profile is for Kubernetes %s, not %s", cached.KubernetesVersion, constant.KubernetesMajorMinorVersion)
	}

	return FromConfigMapData(cached.Data)
}

func loadAPIServerAddressesFromCache(log logrus.FieldLogger, cacheDir string) (addresses []string) {
	lastProfile, err := loadFromCacheDir(cacheDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("Failed to load previous worker configuration")
		}

		return nil
	}

	profile, err := FromConfigMapData(lastProfile.Data)
	if err != nil {
		log.WithError(err).Errorf("Failed to parse previous worker profile %q", lastProfile.Name)
		return nil
	}

	log.Infof("Found previous worker profile %q", lastProfile.Name)
	if !profile.NodeLocalLoadBalancing.IsEnabled() {
		return nil
	}

	for _, address := range profile.APIServerAddresses {
		addresses = append(addresses, address.String())
	}

	return addresses
}

// WatchProfile watches worker configurations stored in Kubernetes based on the
// given worker profile name, invoking callback for every change. Moreover, it
// stores the most recent version of the profile in cacheDir, so that
// [LoadProfile] will find it there. WatchProfile returns with an error if the
// context is done, the callback returns an error, or the watch failed due to
// non-transient errors.
func WatchProfile(ctx context.Context, log logrus.FieldLogger, client kubernetes.Interface, cacheDir, profileName string, callback func(Profile) error) error {
	configMapName := configMapNameForProfile(profileName)

	var lastObservedVersion string
	return watch.ConfigMaps(client.CoreV1().ConfigMaps(metav1.NamespaceSystem)).
		WithObjectName(configMapName).
		WithErrorCallback(func(err error) (time.Duration, error) {
			if retryDelay, e := watch.IsRetryable(err); e == nil {
				log.WithError(err).Debugf(
					"Encountered transient error while watching worker profile %q"+
						", last observed resource version was %q"+
						", retrying in %s",
					profileName, lastObservedVersion, retryDelay,
				)
				return retryDelay, nil
			}
			return 0, err
		}).
		Until(ctx, func(configMap *corev1.ConfigMap) (bool, error) {
			if configMap.ResourceVersion == lastObservedVersion {
				return false, nil
			}

			workerProfile, err := FromConfigMapData(configMap.Data)
			if err != nil {
				return false, err
			}

			if err := storeInCacheDir(cacheDir, newStoredWorkerProfile(profileName, configMap.Data)); err != nil {
				log.WithError(err).Errorf(
					"Failed to write worker profile %q in resource version %q to disk",
					profileName, lastObservedVersion,
				)
			}

			if err := callback(*workerProfile); err != nil {
				return false, err
			}

			lastObservedVersion = configMap.ResourceVersion
			return false, nil
		})
}

func configMapNameForProfile(profileName string) string {
	return strings.Join([]string{
		constant.WorkerConfigComponentName,
		profileName,
		constant.KubernetesMajorMinorVersion,
	}, "-")
}

const cacheFileName = "worker-profile.yaml"

type storedWorkerProfile struct {
	Name string `json:"name"`
	// The Kubernetes minor version this profile has been generated for.
	KubernetesVersion string            `json:"kubernetesVersion,omitempty"`
	Data              map[string]string `json:"data"`
}

func newStoredWorkerProfile(profileName string, data map[string]string) storedWorkerProfile {
	return storedWorkerProfile{
		Name:              profileName,
		KubernetesVersion: constant.KubernetesMajorMinorVersion,
		Data:              data,
	}
}

func loadFromCacheDir(cacheDir string) (*storedWorkerProfile, error) {
	bytes, err := os.ReadFile(filepath.Join(cacheDir, cacheFileName))
	if err != nil {
		return nil, err
	}

	var profile storedWorkerProfile
	err = yaml.Unmarshal(bytes, &profile)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}

func storeInCacheDir(cacheDir string, profile storedWorkerProfile) error {
	bytes, err := yaml.Marshal(&profile)
	if err != nil {
		return err
	}
	return file.WriteContentAtomically(filepath.Join(cacheDir, cacheFileName), bytes, 0644)
}

func loadConcurrently(ctx context.Context, addresses []string, loadWorkerConfig func(context.Context, string) (map[string]string, error)) (map[string]string, error) {
	numAddresses := len(addresses)

	if numAddresses == 1 {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		return loadWorkerConfig(ctx, addresses[0])
	}

	if numAddresses < 1 {
		return nil, nil
	}

	raceCtx, cancelRace := context.WithCancel(context.Background())
	defer cancelRace()

	var ptr atomic.Pointer[map[string]string]
	errs := make([]error, numAddresses)
	var numActiveLoaders atomic.Int64
	numActiveLoaders.Store(int64(numAddresses))

	for pos, address := range addresses {
		go func() {
			defer func() {
				if numActiveLoaders.Add(-1) == 0 {
					cancelRace()
				}
			}()

			if workerConfig, err := loadWorkerConfig(raceCtx, address); err != nil {
				errs[pos] = err
			} else if ptr.CompareAndSwap(nil, &workerConfig) {
				cancelRace()
			}
		}()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-raceCtx.Done():
	}

	if workerConfigPtr := ptr.Load(); workerConfigPtr != nil {
		return *workerConfigPtr, nil
	}

	return nil, errors.Join(errs...)
}
