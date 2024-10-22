/*
Copyright 2024 k0s authors

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

package supervisor

// A handle to a running process. May be used to inspect the process properties
// and terminate it.
type procHandle interface {
	// Reads and returns the process's command line.
	cmdline() ([]string, error)

	// Reads and returns the process's environment.
	environ() ([]string, error)

	// Terminates the process gracefully.
	terminateGracefully() error

	// Terminates the process forcibly.
	terminateForcibly() error
}
