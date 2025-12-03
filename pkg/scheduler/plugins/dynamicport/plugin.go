package dynamicport

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/klog/v2"
)

const (
	Name              = "DynamicPort"
	PortRangeStart    = 20000
	PortRangeEnd      = 32767
	AnnotationKey     = "k0s.io/dynamic-port-count" // User sets this
	AllocatedPortsKey = "DYNAMIC_TCP_PORTS"         // We set this
)

type DynamicPort struct {
	handle framework.Handle
	mu     sync.Mutex
}

var _ framework.ReservePlugin = &DynamicPort{}

func New(_ context.Context, _ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &DynamicPort{handle: h}, nil
}

func (pl *DynamicPort) Name() string {
	return Name
}

func (pl *DynamicPort) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	// Check if pod needs dynamic ports
	countStr, ok := p.Annotations[AnnotationKey]
	if !ok {
		return framework.NewStatus(framework.Success, "")
	}

	// For simplicity, assume count is 1 if present, or parse it. 
	// The article implies getting ports based on need.
	// Let's implement allocation of 1 port for demo purposes if the annotation is present.
	// Or parse "1", "2", etc.
	
	// Logic:
	// 1. List all pods on the node.
	// 2. Collect used ports from their annotations.
	// 3. Pick a random free port.
	// 4. Update Pod annotation.

	nodeInfo, err := pl.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get node info: %v", err))
	}

	usedPorts := make(map[int]bool)
	for _, pod := range nodeInfo.Pods {
		if val, ok := pod.Pod.Annotations[AllocatedPortsKey]; ok {
			// Parse "20000,20001"
			parts := strings.Split(val, ",")
			for _, part := range parts {
				var port int
				if _, err := fmt.Sscanf(part, "%d", &port); err == nil {
					usedPorts[port] = true
				}
			}
		}
	}

	// Simple random allocation with retries
	var allocated int
	found := false
	for i := 0; i < 100; i++ {
		candidate := rand.Intn(PortRangeEnd-PortRangeStart) + PortRangeStart
		if !usedPorts[candidate] {
			allocated = candidate
			found = true
			break
		}
	}

	if !found {
		return framework.NewStatus(framework.Unschedulable, "no free dynamic ports available")
	}

	// Apply to Pod
	// Note: modifying the Pod in Reserve is tricky because we need to persist it.
	// The Scheduler Framework doesn't persist changes to the Pod in API server automatically.
	// We must use the Client.
	
	newPod := p.DeepCopy()
	if newPod.Annotations == nil {
		newPod.Annotations = make(map[string]string)
	}
	newPod.Annotations[AllocatedPortsKey] = fmt.Sprintf("%d", allocated)

	_, err = pl.handle.ClientSet().CoreV1().Pods(p.Namespace).Update(ctx, newPod, metav1.UpdateOptions{})
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to update pod annotations: %v", err))
	}
	
	klog.V(2).Infof("Allocated dynamic port %d for pod %s on node %s", allocated, p.Name, nodeName)

	return framework.NewStatus(framework.Success, "")
}

func (pl *DynamicPort) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	// Ideally we should clean up the annotation if scheduling fails later.
	// But for this PoC, we assume it's fine (will be overwritten or pod deleted).
}
