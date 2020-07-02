package applier

import (
	"sigs.k8s.io/cli-utils/pkg/apply/event"
)

type Printer struct {
	Name string
}

func (p *Printer) Print(ch <-chan event.Event, preview bool) {

}
