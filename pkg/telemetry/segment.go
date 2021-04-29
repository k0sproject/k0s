package telemetry

import (
	"github.com/segmentio/analytics-go"
)

var segmentToken = ""

const heartbeatEvent = "cluster-heartbeat"

// Analytics is the interface used for our analytics client.
type analyticsClient interface {
	Enqueue(msg analytics.Message) error
	Close() error
}

func newSegmentClient(segmentToken string) analyticsClient {
	return analytics.New(segmentToken)
}
