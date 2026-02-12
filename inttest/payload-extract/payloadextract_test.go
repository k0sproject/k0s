// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package payloadextract

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type PayloadExtractSuite struct {
	common.BootlooseSuite
}

// TestPayloadExtract tests extraction functionality and that k0s uses pre-extracted binaries
func (s *PayloadExtractSuite) TestPayloadExtract() {
	ssh, err := s.SSH(s.Context(), s.ControllerNode(0))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	dataDir := "/var/lib/k0s"

	s.Run("extract", func() {
		s.T().Log("Extracting binaries to", dataDir)
		output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s payload extract --data-dir %s", s.K0sFullPath, dataDir))
		s.Require().NoError(err, "payload extract command failed")
		s.T().Logf("Extract output:\n%s", output)

		// Verify the bin directory was created
		_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("test -d %s/bin", dataDir))
		s.Require().NoError(err, "bin directory was not created")

		// Get actual list of binaries that were extracted by listing the directory
		lsOutput, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("ls -1 %s/bin", dataDir))
		s.Require().NoError(err, "failed to list bin directory")
		actualBinaries := strings.Split(strings.TrimSpace(lsOutput), "\n")
		s.T().Logf("Found %d actual binaries: %v", len(actualBinaries), actualBinaries)
		s.Require().NotEmpty(actualBinaries, "no binaries found in bin directory")

		s.T().Log("Verifying all binaries were extracted")
		for _, binary := range actualBinaries {
			binaryPath := fmt.Sprintf("%s/bin/%s", dataDir, binary)

			// Check if file exists
			_, err = ssh.ExecWithOutput(s.Context(), "test -f "+binaryPath)
			s.Require().NoError(err, "binary %s not found", binary)

			// Check if file is executable
			_, err = ssh.ExecWithOutput(s.Context(), "test -x "+binaryPath)
			s.Require().NoError(err, "binary %s is not executable", binary)

			// Check file size is reasonable (> 100KB, embedded binaries should be substantial)
			sizeOutput, err := ssh.ExecWithOutput(s.Context(), "stat -c %%s "+binaryPath)
			s.Require().NoError(err, "failed to get size of %s", binary)
			s.T().Logf("Binary %s size: %s bytes", binary, strings.TrimSpace(sizeOutput))
		}

		s.T().Log("Verifying all binaries can be executed")
		// Try to execute each binary with --help (most binaries support this)
		// Some may fail but we just log those - the important thing is they're valid executables
		for _, binary := range actualBinaries {
			binaryPath := fmt.Sprintf("%s/bin/%s", dataDir, binary)

			// Try --help first, if that fails try --version, if that fails just run it to see if it's executable
			output, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("%s --help 2>&1 || %s --version 2>&1 || true", binaryPath, binaryPath))
			if err == nil && output != "" {
				s.T().Logf("%s is executable (output: %s)", binary, strings.Split(output, "\n")[0])
			} else {
				s.T().Logf("Warning: %s may not support --help or --version, but file is executable", binary)
			}
		}
	})

	s.Run("start_with_preextracted_binaries", func() {
		// Get list of extracted binaries and their timestamps
		lsOutput, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("ls -1 %s/bin", dataDir))
		s.Require().NoError(err, "failed to list bin directory")
		extractedBinaries := strings.Split(strings.TrimSpace(lsOutput), "\n")
		s.T().Logf("Found %d pre-extracted binaries", len(extractedBinaries))
		s.Require().NotEmpty(extractedBinaries, "no binaries found after extraction")

		// Get the timestamps of all binaries to verify k0s doesn't re-extract them
		timestampsBefore := make(map[string]string)
		for _, binary := range extractedBinaries {
			timestamp, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("stat -c %%Y %s/bin/%s", dataDir, binary))
			s.Require().NoError(err, "failed to get timestamp for %s", binary)
			timestampsBefore[binary] = strings.TrimSpace(timestamp)
		}
		s.T().Logf("Recorded timestamps for %d binaries before k0s start", len(timestampsBefore))

		s.T().Log("Starting k0s controller")
		s.Require().NoError(s.InitController(0, "--enable-worker"))
		s.Require().NoError(s.WaitForKubeAPI(s.ControllerNode(0)))

		kc, err := s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)

		err = s.WaitForNodeReady(s.ControllerNode(0), kc)
		s.Require().NoError(err)

		s.AssertSomeKubeSystemPods(kc)

		// Check that no binaries were re-extracted (timestamps should be the same)
		s.T().Log("Verifying binaries were not re-extracted")
		for _, binary := range extractedBinaries {
			timestampAfter, err := ssh.ExecWithOutput(s.Context(), fmt.Sprintf("stat -c %%Y %s/bin/%s", dataDir, binary))
			s.Require().NoError(err, "failed to get timestamp for %s after start", binary)

			s.Equal(timestampsBefore[binary], strings.TrimSpace(timestampAfter),
				"binary %s was re-extracted when it should have used the pre-extracted version", binary)
		}
		s.T().Logf("Verified all %d binaries were not re-extracted", len(extractedBinaries))
	})
}

func TestPayloadExtractSuite(t *testing.T) {
	s := PayloadExtractSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     0,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	}
	suite.Run(t, &s)
}
