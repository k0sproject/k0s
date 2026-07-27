// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package basic

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	clientmodel "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/stretchr/testify/assert"
)

var goldenCAdvisorMetrics = [][]string{
	{"cadvisor_version_info", "kernelVersion", "osVersion"},
	// {"container_blkio_device_usage_total", "container", "device", "id", "image", "major", "minor", "name", "namespace", "operation", "pod"},
	{"container_blkio_device_usage_total", "device", "id", "major", "minor", "namespace", "operation", "pod"},
	{"container_blkio_device_usage_total", "device", "id", "major", "minor", "operation"},
	{"container_cpu_load_average_10s", "container", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_load_average_10s", "id"},
	{"container_cpu_load_average_10s", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_load_average_10s", "id", "namespace", "pod"},
	{"container_cpu_load_d_average_10s", "container", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_load_d_average_10s", "id"},
	{"container_cpu_load_d_average_10s", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_load_d_average_10s", "id", "namespace", "pod"},
	{"container_cpu_system_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_system_seconds_total", "id"},
	{"container_cpu_system_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_system_seconds_total", "id", "namespace", "pod"},
	{"container_cpu_usage_seconds_total", "container", "cpu", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_usage_seconds_total", "cpu", "id"},
	{"container_cpu_usage_seconds_total", "cpu", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_usage_seconds_total", "cpu", "id", "namespace", "pod"},
	{"container_cpu_user_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_user_seconds_total", "id"},
	{"container_cpu_user_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_cpu_user_seconds_total", "id", "namespace", "pod"},
	{"container_creation_time_seconds", "container", "id", "image", "name", "namespace", "pod"},
	{"container_creation_time_seconds", "id"},
	{"container_creation_time_seconds", "id", "image", "name", "namespace", "pod"},
	{"container_creation_time_seconds", "id", "namespace", "pod"},
	{"container_file_descriptors", "container", "id", "image", "name", "namespace", "pod"},
	{"container_file_descriptors", "id"},
	{"container_file_descriptors", "id", "image", "name", "namespace", "pod"},
	{"container_file_descriptors", "id", "namespace", "pod"},
	{"container_fs_inodes_free", "device", "id"},
	{"container_fs_inodes_total", "device", "id"},
	{"container_fs_io_current", "device", "id"},
	{"container_fs_io_time_seconds_total", "device", "id"},
	{"container_fs_io_time_weighted_seconds_total", "device", "id"},
	{"container_fs_limit_bytes", "device", "id"},
	{"container_fs_read_seconds_total", "device", "id"},
	// {"container_fs_reads_bytes_total", "container", "device", "id", "image", "name", "namespace", "pod"},
	{"container_fs_reads_bytes_total", "device", "id"},
	{"container_fs_reads_bytes_total", "device", "id", "namespace", "pod"},
	{"container_fs_reads_merged_total", "device", "id"},
	// {"container_fs_reads_total", "container", "device", "id", "image", "name", "namespace", "pod"},
	{"container_fs_reads_total", "device", "id"},
	{"container_fs_reads_total", "device", "id", "namespace", "pod"},
	{"container_fs_sector_reads_total", "device", "id"},
	{"container_fs_sector_writes_total", "device", "id"},
	{"container_fs_usage_bytes", "device", "id"},
	{"container_fs_write_seconds_total", "device", "id"},
	// {"container_fs_writes_bytes_total", "container", "device", "id", "image", "name", "namespace", "pod"},
	{"container_fs_writes_bytes_total", "device", "id"},
	{"container_fs_writes_bytes_total", "device", "id", "namespace", "pod"},
	{"container_fs_writes_merged_total", "device", "id"},
	// {"container_fs_writes_total", "container", "device", "id", "image", "name", "namespace", "pod"},
	{"container_fs_writes_total", "device", "id"},
	{"container_fs_writes_total", "device", "id", "namespace", "pod"},
	{"container_health_state", "container", "id", "image", "name", "namespace", "pod"},
	{"container_health_state", "id"},
	{"container_health_state", "id", "image", "name", "namespace", "pod"},
	{"container_health_state", "id", "namespace", "pod"},
	{"container_last_seen", "container", "id", "image", "name", "namespace", "pod"},
	{"container_last_seen", "id"},
	{"container_last_seen", "id", "image", "name", "namespace", "pod"},
	{"container_last_seen", "id", "namespace", "pod"},
	{"container_memory_cache", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_cache", "id"},
	{"container_memory_cache", "id", "image", "name", "namespace", "pod"},
	{"container_memory_cache", "id", "namespace", "pod"},
	{"container_memory_events_high_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_events_high_total", "id"},
	{"container_memory_events_high_total", "id", "image", "name", "namespace", "pod"},
	{"container_memory_events_high_total", "id", "namespace", "pod"},
	{"container_memory_events_max_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_events_max_total", "id"},
	{"container_memory_events_max_total", "id", "image", "name", "namespace", "pod"},
	{"container_memory_events_max_total", "id", "namespace", "pod"},
	{"container_memory_failcnt", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_failcnt", "id"},
	{"container_memory_failcnt", "id", "image", "name", "namespace", "pod"},
	{"container_memory_failcnt", "id", "namespace", "pod"},
	{"container_memory_failures_total", "container", "failure_type", "id", "image", "name", "namespace", "pod", "scope"},
	{"container_memory_failures_total", "failure_type", "id", "image", "name", "namespace", "pod", "scope"},
	{"container_memory_failures_total", "failure_type", "id", "namespace", "pod", "scope"},
	{"container_memory_failures_total", "failure_type", "id", "scope"},
	{"container_memory_kernel_usage", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_kernel_usage", "id"},
	{"container_memory_kernel_usage", "id", "image", "name", "namespace", "pod"},
	{"container_memory_kernel_usage", "id", "namespace", "pod"},
	{"container_memory_mapped_file", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_mapped_file", "id"},
	{"container_memory_mapped_file", "id", "image", "name", "namespace", "pod"},
	{"container_memory_mapped_file", "id", "namespace", "pod"},
	{"container_memory_max_usage_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_max_usage_bytes", "id"},
	{"container_memory_max_usage_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_memory_max_usage_bytes", "id", "namespace", "pod"},
	{"container_memory_rss", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_rss", "id"},
	{"container_memory_rss", "id", "image", "name", "namespace", "pod"},
	{"container_memory_rss", "id", "namespace", "pod"},
	{"container_memory_swap", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_swap", "id"},
	{"container_memory_swap", "id", "image", "name", "namespace", "pod"},
	{"container_memory_swap", "id", "namespace", "pod"},
	{"container_memory_total_active_file_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_total_active_file_bytes", "id"},
	{"container_memory_total_active_file_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_memory_total_active_file_bytes", "id", "namespace", "pod"},
	{"container_memory_total_inactive_file_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_total_inactive_file_bytes", "id"},
	{"container_memory_total_inactive_file_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_memory_total_inactive_file_bytes", "id", "namespace", "pod"},
	{"container_memory_usage_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_usage_bytes", "id"},
	{"container_memory_usage_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_memory_usage_bytes", "id", "namespace", "pod"},
	{"container_memory_working_set_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_memory_working_set_bytes", "id"},
	{"container_memory_working_set_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_memory_working_set_bytes", "id", "namespace", "pod"},
	{"container_network_receive_bytes_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_receive_bytes_total", "id", "interface"},
	{"container_network_receive_errors_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_receive_errors_total", "id", "interface"},
	{"container_network_receive_packets_dropped_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_receive_packets_dropped_total", "id", "interface"},
	{"container_network_receive_packets_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_receive_packets_total", "id", "interface"},
	{"container_network_transmit_bytes_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_transmit_bytes_total", "id", "interface"},
	{"container_network_transmit_errors_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_transmit_errors_total", "id", "interface"},
	{"container_network_transmit_packets_dropped_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_transmit_packets_dropped_total", "id", "interface"},
	{"container_network_transmit_packets_total", "id", "image", "interface", "name", "namespace", "pod"},
	{"container_network_transmit_packets_total", "id", "interface"},
	{"container_oom_events_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_oom_events_total", "id"},
	{"container_oom_events_total", "id", "image", "name", "namespace", "pod"},
	{"container_oom_events_total", "id", "namespace", "pod"},
	{"container_pressure_cpu_stalled_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_cpu_stalled_seconds_total", "id"},
	{"container_pressure_cpu_stalled_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_cpu_stalled_seconds_total", "id", "namespace", "pod"},
	{"container_pressure_cpu_waiting_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_cpu_waiting_seconds_total", "id"},
	{"container_pressure_cpu_waiting_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_cpu_waiting_seconds_total", "id", "namespace", "pod"},
	{"container_pressure_io_stalled_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_io_stalled_seconds_total", "id"},
	{"container_pressure_io_stalled_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_io_stalled_seconds_total", "id", "namespace", "pod"},
	{"container_pressure_io_waiting_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_io_waiting_seconds_total", "id"},
	{"container_pressure_io_waiting_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_io_waiting_seconds_total", "id", "namespace", "pod"},
	{"container_pressure_memory_stalled_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_memory_stalled_seconds_total", "id"},
	{"container_pressure_memory_stalled_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_memory_stalled_seconds_total", "id", "namespace", "pod"},
	{"container_pressure_memory_waiting_seconds_total", "container", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_memory_waiting_seconds_total", "id"},
	{"container_pressure_memory_waiting_seconds_total", "id", "image", "name", "namespace", "pod"},
	{"container_pressure_memory_waiting_seconds_total", "id", "namespace", "pod"},
	{"container_processes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_processes", "id"},
	{"container_processes", "id", "image", "name", "namespace", "pod"},
	{"container_processes", "id", "namespace", "pod"},
	{"container_scrape_error"},
	{"container_sockets", "container", "id", "image", "name", "namespace", "pod"},
	{"container_sockets", "id"},
	{"container_sockets", "id", "image", "name", "namespace", "pod"},
	{"container_sockets", "id", "namespace", "pod"},
	{"container_spec_cpu_period", "container", "id", "image", "name", "namespace", "pod"},
	{"container_spec_cpu_period", "id"},
	{"container_spec_cpu_period", "id", "image", "name", "namespace", "pod"},
	{"container_spec_cpu_period", "id", "namespace", "pod"},
	{"container_spec_cpu_shares", "container", "id", "image", "name", "namespace", "pod"},
	{"container_spec_cpu_shares", "id"},
	{"container_spec_cpu_shares", "id", "image", "name", "namespace", "pod"},
	{"container_spec_cpu_shares", "id", "namespace", "pod"},
	{"container_spec_memory_limit_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_limit_bytes", "id"},
	{"container_spec_memory_limit_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_limit_bytes", "id", "namespace", "pod"},
	{"container_spec_memory_reservation_limit_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_reservation_limit_bytes", "id"},
	{"container_spec_memory_reservation_limit_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_reservation_limit_bytes", "id", "namespace", "pod"},
	{"container_spec_memory_swap_limit_bytes", "container", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_swap_limit_bytes", "id"},
	{"container_spec_memory_swap_limit_bytes", "id", "image", "name", "namespace", "pod"},
	{"container_spec_memory_swap_limit_bytes", "id", "namespace", "pod"},
	{"container_start_time_seconds", "container", "id", "image", "name", "namespace", "pod"},
	{"container_start_time_seconds", "id"},
	{"container_start_time_seconds", "id", "image", "name", "namespace", "pod"},
	{"container_start_time_seconds", "id", "namespace", "pod"},
	{"container_tasks_state", "container", "id", "image", "name", "namespace", "pod", "state"},
	{"container_tasks_state", "id", "image", "name", "namespace", "pod", "state"},
	{"container_tasks_state", "id", "namespace", "pod", "state"},
	{"container_tasks_state", "id", "state"},
	{"container_threads", "container", "id", "image", "name", "namespace", "pod"},
	{"container_threads", "id"},
	{"container_threads", "id", "image", "name", "namespace", "pod"},
	{"container_threads", "id", "namespace", "pod"},
	{"container_threads_max", "container", "id", "image", "name", "namespace", "pod"},
	{"container_threads_max", "id"},
	{"container_threads_max", "id", "image", "name", "namespace", "pod"},
	{"container_threads_max", "id", "namespace", "pod"},
	{"container_ulimits_soft", "container", "id", "image", "name", "namespace", "pod", "ulimit"},
	{"container_ulimits_soft", "id", "image", "name", "namespace", "pod", "ulimit"},
	{"container_ulimits_soft", "id", "ulimit"},
	{"machine_cpu_books", "boot_id", "system_uuid"},
	{"machine_cpu_cores", "boot_id", "system_uuid"},
	{"machine_cpu_drawers", "boot_id", "system_uuid"},
	{"machine_cpu_physical_cores", "boot_id", "system_uuid"},
	{"machine_cpu_sockets", "boot_id", "system_uuid"},
	{"machine_memory_bytes", "boot_id", "system_uuid"},
	{"machine_nvm_avg_power_budget_watts", "boot_id", "system_uuid"},
	{"machine_nvm_capacity", "boot_id", "mode", "system_uuid"},
	{"machine_scrape_error"},
	{"machine_swap_bytes", "boot_id", "system_uuid"},
}

var expectedContainerImages = []struct{ container, name, version string }{
	{"coredns", constant.CoreDNSImage, constant.CoreDNSImageVersion},
	{"konnectivity-agent", constant.KonnectivityImage, constant.KonnectivityImageVersion},
	{"kube-proxy", constant.KubeProxyImage, constant.KubeProxyImageVersion},
	{"kube-router", constant.KubeRouterCNIImage, constant.KubeRouterCNIImageVersion},
	{"metrics-server", constant.MetricsImage, constant.MetricsImageVersion},
}

type failures []string

func (f *failures) Errorf(format string, args ...any) { *f = append(*f, fmt.Sprintf(format, args...)) }
func (f *failures) Failed() bool                      { return len(*f) > 0 }
func (f *failures) Reset()                            { *f = nil }

func verifyCAdvisorMetrics(ctx context.Context, t *testing.T, client kubernetes.Interface, node string) bool {
	t.Helper()

	req := client.CoreV1().RESTClient().Get().
		Resource("nodes").Name(node).
		SubResource("proxy", "metrics", "cadvisor").
		Param("format", "text")

	var f failures
	for attempt := uint(1); ; attempt++ {
		if attempt > 1 {
			select {
			case <-time.After(wait.Jitter(800*time.Millisecond, 0.5)):
				if attempt%20 == 0 {
					t.Log("Still trying to verify cAdvisor metrics on", node)
				}
				f.Reset()

			case <-ctx.Done():
				assert.Fail(t, "Interrupted", "While verifying cAdvisor metrics on %v", node)
				for _, reason := range f {
					t.Log(reason)
				}
				return false
			}
		}

		stream, err := req.Stream(ctx)
		if !assert.NoErrorf(&f, err, "While streaming cAdvisor metrics from %s", node) {
			continue
		}

		parser := expfmt.NewTextParser(model.UTF8Validation)
		families, err := parser.TextToMetricFamilies(stream)
		if !assert.NoErrorf(&f, errors.Join(err, stream.Close()), "While parsing cAdvisor metrics on %s", node) {
			continue
		}

		allMetrics := collectAllMetrics(families)
		missingMetrics := slices.DeleteFunc(slices.Clone(goldenCAdvisorMetrics), func(c []string) bool {
			_, found := slices.BinarySearchFunc(allMetrics, c, slices.Compare)
			return found
		})
		assert.Empty(&f, missingMetrics, "Some expected cAdvisor metrics are missing")

		verifyContainerMetricsImageLabels(&f, families)

		if !f.Failed() {
			return true
		}
	}
}

// Extracts all non-empty metric/label combinations, sorted lexicographically.
func collectAllMetrics(families map[string]*clientmodel.MetricFamily) (all [][]string) {
	names := slices.Collect(maps.Keys(families))
	slices.Sort(names)

	for _, name := range names {
		off := len(all)
		for _, metric := range families[name].Metric {
			labels := make([]string, 1, len(metric.Label)+1)
			labels[0] = name

			for name, value := range nonNilLabels(metric.Label) {
				if value != "" {
					labels = append(labels, name)
				}
			}
			slices.Sort(labels[1:])
			if !slices.ContainsFunc(all[off:], func(c []string) bool {
				return slices.Equal(c[1:], labels[1:])
			}) {
				all = append(all, labels)
			}
		}

		slices.SortFunc(all[off:], slices.Compare)
	}

	return all
}

func verifyContainerMetricsImageLabels(t assert.TestingT, families map[string]*clientmodel.MetricFamily) {
	// Select all metric families that have a container label.
	var names []string
	for name := range families {
		if slices.ContainsFunc(goldenCAdvisorMetrics, func(c []string) (found bool) {
			if c[0] == name {
				_, found = slices.BinarySearch(c[1:], "container")
			}
			return found
		}) {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	// Determine the expected container image labels. If hashes are used, tags are stripped.
	expectedContainers := make(map[string]string, len(expectedContainerImages))
	for _, containerImage := range expectedContainerImages {
		suffix := ":" + containerImage.version
		if idx := strings.Index(suffix, "@"); idx >= 0 {
			suffix = suffix[idx:]
		}
		expectedContainers[containerImage.container] = containerImage.name + suffix
	}

	// Check that each family has the right metrics for each of the expected containers.
	for _, familyName := range names {
		expectedContainers := maps.Clone(expectedContainers)
		seenContainers := make(map[string]string, len(expectedContainers))

		for _, metric := range families[familyName].Metric {
			var container, image string
			for name, value := range nonNilLabels(metric.Label) {
				switch name {
				case "container":
					container = value
				case "image":
					image = value
				}
			}
			if container == "" || image == "" {
				continue
			}

			if seenImage, seen := seenContainers[container]; seen {
				assert.Equalf(t, seenImage, image, "Multiple images for %s for %s", container, familyName)
				continue
			}
			seenContainers[container] = image

			expectedImage, containerExpected := expectedContainers[container]
			if assert.Truef(t, containerExpected, "Unexpected container %s with image %s for %s", container, image, familyName) {
				assert.Equalf(t, expectedImage, image, "Unexpected image for %s for %s", container, familyName)
				delete(expectedContainers, container)
			}
		}

		// metrics-server is running as a single-pod deployment, so it may or may not be present
		delete(expectedContainers, "metrics-server")
		missingContainers := slices.Collect(maps.Keys(expectedContainers))
		slices.Sort(missingContainers)
		assert.Emptyf(t, missingContainers, "Some containers are missing for %s", familyName)
	}
}

func nonNilLabels(pairs []*clientmodel.LabelPair) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for _, pair := range pairs {
			if n, v := pair.Name, pair.Value; n != nil && v != nil && *n != "" {
				if !yield(*n, *v) {
					return
				}
			}
		}
	}
}
