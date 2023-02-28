//go:build windows
// +build windows

/*
Copyright 2020 k0s authors

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

import (
	"time"
)

// maybeKillPidFile checks kills the process in the pidFile if it's has
// the same binary as the supervisor's. This function does not delete
// the old pidFile as this is done by the caller.
func (s *Supervisor) maybeKillPidFile(check <-chan time.Time, deadline <-chan time.Time) error {
	s.log.Warnf("maybeKillPidFile is not implemented on Windows")
	return nil
}
