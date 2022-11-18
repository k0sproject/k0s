package prober

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

type mockComponent struct {
	counter int
	errors  []error
}

func (mc *mockComponent) Healthy() error {
	if mc.counter >= len(mc.errors) {
		return nil
	}
	err := mc.errors[mc.counter]
	mc.counter++
	return err
}
func TestProber(t *testing.T) {
	testProber := func(iterations int) *Prober {
		p := New()
		p.interval = 1 * time.Millisecond
		p.probesTrackLength = 3
		p.stopAfterIterationNum = iterations
		l, _ := test.NewNullLogger()
		p.l = l.WithField("test", "prober")
		return p
	}

	t.Run("prober_stores_not_more_than_n_last_results_for_each_component", func(t *testing.T) {
		prober := testProber(9)
		prober.Register("test", &mockComponent{})
		prober.Register("test2", &mockComponent{})
		prober.Run(context.Background())
		st := prober.State()

		assert.Len(t, st, 2, "should have 2 components in the state")
		assert.Len(t, st["test"], 3, "should have 3 results for test component even after 9 iterations")
		assert.Len(t, st["test2"], 3, "should have 3 results for test2 component even after 9 iterations")
	})

	t.Run("prober_stores_and_overrides_error_results", func(t *testing.T) {
		prober := testProber(5)

		prober.Register("test", &mockComponent{
			errors: []error{nil, nil, fmt.Errorf("test1 error"), nil, nil},
		})

		prober.Register("test2", &mockComponent{
			errors: []error{nil, fmt.Errorf("test2 error"), nil, nil, nil},
		})

		prober.Register("test3", &mockComponent{
			errors: []error{nil, nil, nil, nil, fmt.Errorf("test3 error")},
		})
		prober.Run(context.Background())
		st := prober.State()

		assert.Len(t, st, 3, "should have 3 components in the state")
		assert.Len(t, st["test"], 3, "should have 5 result for test component")
		assert.Len(t, st["test2"], 3, "should have 5 result for test2 component")

		assert.Error(t, st["test"][0].Error, "should have error in the first result for component `test`")
		assert.NoError(t, st["test2"][0].Error, "should have no errors for component `test2`")
		assert.NoError(t, st["test2"][1].Error, "should have no errors for component `test2`")
		assert.NoError(t, st["test2"][2].Error, "should have no errors for component `test2`")
		assert.Error(t, st["test3"][2].Error, "should have error in the last result for component `test3`")
	})

}
