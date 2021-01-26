package supervisor

import "testing"

type SupervisorTest struct {
	shouldFail bool
	proc       Supervisor
}

func TestSupervisor(t *testing.T) {
	var testSupervisors = []SupervisorTest{
		SupervisorTest{
			shouldFail: false,
			proc: Supervisor{
				Name:    "supervisor-test-sleep",
				BinPath: "/bin/sh",
				RunDir:  ".",
				Args:    []string{"-c", "sleep 1s"},
			},
		},
		SupervisorTest{
			shouldFail: false,
			proc: Supervisor{
				Name:    "supervisor-test-fail",
				BinPath: "/bin/false",
				RunDir:  ".",
			},
		},
		SupervisorTest{
			shouldFail: false,
			proc: Supervisor{
				Name:    "supervisor-test-non-executable",
				BinPath: "/tmp",
				RunDir:  ".",
			},
		},
		SupervisorTest{
			shouldFail: true,
			proc: Supervisor{
				Name:    "supervisor-test-rundir-fail",
				BinPath: "/tmp",
				RunDir:  "/bin/sh/foo/bar",
			},
		},
	}

	for _, s := range testSupervisors {
		err := s.proc.Supervise()
		if err != nil && !s.shouldFail {
			t.Errorf("Failed to start %s: %w", s.proc.Name, err)
		} else if err == nil && s.shouldFail {
			t.Errorf("%s should fail but didn't", s.proc.Name)
		}
		err = s.proc.Stop()
		if err != nil {
			t.Errorf("Failed to stop %s: %w", s.proc.Name, err)
		}
	}
}
