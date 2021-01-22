package supervisor

import "testing"

func TestSupervisor(t *testing.T) {
	var testSupervisors = []Supervisor{
		Supervisor{
			Name:    "supervisor-test-sleep",
			BinPath: "/bin/sh",
			RunDir:  ".",
			Args:    []string{"-c", "sleep 1s"},
		},
		Supervisor{
			Name:    "supervisor-test-fail",
			BinPath: "/bin/false",
			RunDir:  ".",
		},
		Supervisor{
			Name:    "supervisor-test-non-executable",
			BinPath: "/tmp",
			RunDir:  ".",
		},
	}

	for _, s := range testSupervisors {
		err := s.Supervise()
		if err != nil {
			t.Errorf("Failed to start %s: %w", s.Name, err)
		}
		err = s.Stop()
		if err != nil {
			t.Errorf("Failed to stop %s: %w", s.Name, err)
		}
	}
}
