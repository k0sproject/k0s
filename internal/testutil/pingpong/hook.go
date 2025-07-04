/*
Copyright 2025 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
