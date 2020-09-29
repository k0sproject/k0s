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
