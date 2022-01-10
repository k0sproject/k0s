/*
Copyright 2022 k0s authors

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
package sonobuoy

import (
	"bufio"
	"strconv"
	"strings"
)

// Result defines the results of a sonobuoy run
type Result struct {
	Plugin  string
	Status  string
	Total   int
	Passed  int
	Failed  int
	Skipped int

	ResultPath string
}

// ResultFromString constructs the Result struct from "sonobuoy results ..." output
func ResultFromString(output string) (Result, error) {
	result := Result{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		vals := strings.Split(scanner.Text(), ":")
		switch vals[0] {
		case "Plugin":
			result.Plugin = strings.TrimSpace(vals[1])
		case "Status":
			result.Status = strings.TrimSpace(vals[1])
		case "Total":
			val, err := strconv.Atoi(strings.TrimSpace(vals[1]))
			if err != nil {
				return result, err
			}
			result.Total = val
		case "Passed":
			val, err := strconv.Atoi(strings.TrimSpace(vals[1]))
			if err != nil {
				return result, err
			}
			result.Passed = val
		case "Failed":
			val, err := strconv.Atoi(strings.TrimSpace(vals[1]))
			if err != nil {
				return result, err
			}
			result.Failed = val
		case "Skipped":
			val, err := strconv.Atoi(strings.TrimSpace(vals[1]))
			if err != nil {
				return result, err
			}
			result.Skipped = val
		}

	}
	return result, nil
}
