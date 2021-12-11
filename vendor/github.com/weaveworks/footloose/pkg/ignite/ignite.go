package ignite

import (
	"fmt"

	"github.com/blang/semver"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/footloose/pkg/exec"
)

const execName = "ignite"

var minVersion = semver.MustParse("0.5.2") // Require v0.5.2 or higher

func CheckVersion() {

	lines, err := exec.CombinedOutputLines(exec.Command(execName, "version", "-o", "short"))
	if err == nil && len(lines) == 0 {
		err = fmt.Errorf("no output")
	}

	if err != nil {
		verParseFail(err)
	}

	// Use ParseTolerant as Ignite's version has a leading "v"
	version, err := semver.ParseTolerant(lines[0])
	if err != nil {
		verParseFail(err)
	}

	if minVersion.Compare(version) > 0 {
		verParseFail(fmt.Errorf("minimum version is v%s, detected older v%s", minVersion, version))
	}

	if len(version.Build) > 0 {
		log.Warnf("Continuing with a dirty build of Ignite (v%s), here be dragons", version)
	}
}

func verParseFail(err error) {
	log.Fatalf("Failed to verify Ignite version: %v", err)
}
