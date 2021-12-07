package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/weaveworks/footloose/pkg/config"
)

// Formatter formats a slice of machines and outputs the result
// in a given format.
type Formatter interface {
	Format(io.Writer, []*Machine) error
}

// JSONFormatter formats a slice of machines into a JSON and
// outputs it to stdout.
type JSONFormatter struct{}

// TableFormatter formats a slice of machines into a colored
// table like output and prints that to stdout.
type TableFormatter struct{}

type port struct {
	Guest int `json:"guest"`
	Host  int `json:"host"`
}

const (
	// NotCreated status of a machine
	NotCreated = "Not created"
	// Stopped status of a machine
	Stopped = "Stopped"
	// Running status of a machine
	Running = "Running"
)

// MachineStatus is the runtime status of a Machine.
type MachineStatus struct {
	Container       string            `json:"container"`
	State           string            `json:"state"`
	Spec            *config.Machine   `json:"spec,omitempty"`
	Ports           []port            `json:"ports"`
	Hostname        string            `json:"hostname"`
	Image           string            `json:"image"`
	Command         string            `json:"cmd"`
	IP              string            `json:"ip"`
	RuntimeNetworks []*RuntimeNetwork `json:"runtimeNetworks,omitempty"`
}

// Format will output to stdout in JSON format.
func (JSONFormatter) Format(w io.Writer, machines []*Machine) error {
	var statuses []MachineStatus
	for _, m := range machines {
		statuses = append(statuses, *m.Status())
	}

	m := struct {
		Machines []MachineStatus `json:"machines"`
	}{
		Machines: statuses,
	}
	ms, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	ms = append(ms, '\n')
	_, err = w.Write(ms)
	return err
}

// FormatSingle is a json formatter for a single machine.
func (JSONFormatter) FormatSingle(w io.Writer, m *Machine) error {
	status, err := json.MarshalIndent(m.Status(), "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(status)
	return err
}

// writer contains writeColumns' error value to clean-up some error handling
type writer struct {
	err error
}

// writerColumns is a no-op if there was an error already
func (wr writer) writeColumns(w io.Writer, cols []string) {
	if wr.err != nil {
		return
	}
	_, err := fmt.Fprintln(w, strings.Join(cols, "\t"))
	wr.err = err
}

// Format will output to stdout in table format.
func (TableFormatter) Format(w io.Writer, machines []*Machine) error {
	const padding = 3
	wr := new(writer)
	var statuses []MachineStatus
	for _, m := range machines {
		statuses = append(statuses, *m.Status())
	}

	table := tabwriter.NewWriter(w, 0, 0, padding, ' ', 0)
	wr.writeColumns(table, []string{"NAME", "HOSTNAME", "PORTS", "IP", "IMAGE", "CMD", "STATE", "BACKEND"})
	// we bail early here if there was an error so we don't process the below loop
	if wr.err != nil {
		return wr.err
	}
	for _, s := range statuses {
		var ports []string
		for k, v := range s.Ports {
			p := fmt.Sprintf("%d->%d", k, v)
			ports = append(ports, p)
		}
		if len(ports) < 1 {
			for _, p := range s.Spec.PortMappings {
				port := fmt.Sprintf("%d->%d", p.HostPort, p.ContainerPort)
				ports = append(ports, port)
			}
		}
		ps := strings.Join(ports, ",")
		wr.writeColumns(table, []string{s.Container, s.Hostname, ps, s.IP, s.Image, s.Command, s.State, s.Spec.Backend})
	}

	if wr.err != nil {
		return wr.err
	}
	return table.Flush()
}
