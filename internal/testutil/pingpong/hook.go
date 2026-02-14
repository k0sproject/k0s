// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package pingpong

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Hook() {
	pingPongHook()
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "PID", os.Getpid(), "says good bye!")
	os.Exit(0)
}

func runHook(ignoreGracefulTerminationRequests bool, hook func() error) error {
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	if ignoreGracefulTerminationRequests {
		if _, err := fmt.Fprintln(os.Stderr, "Will ignore all graceful termination requests ..."); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintln(os.Stderr, "Responding to graceful termination requests ..."); err != nil {
		return err
	}

	hookResult := make(chan error, 1)
	go func() { hookResult <- hook() }()

	timeout := time.NewTimer(1 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case sig := <-signals:
			if ignoreGracefulTerminationRequests {
				if _, err := fmt.Fprintln(os.Stderr, "Ignoring:", sig); err != nil {
					return err
				}
				break
			}

			fmt.Fprintln(os.Stderr, "Terminating gracefully:", sig)
			return nil

		case err := <-hookResult:
			return err

		case <-timeout.C:
			panic("timeout")
		}
	}
}
