package backup

import (
	"os"
	"path/filepath"
)

type certsStep struct {
	certRootDir string
}

func newCertsStep(certRootDir string) *certsStep {
	return &certsStep{certRootDir: certRootDir}
}

func (c certsStep) Name() string {
	return "certificates"
}

func (c certsStep) Backup(workingDir string) (StepResult, error) {
	// TODO: may be it's better to copy them to temp directory first to avoid any issues with possible races here?
	files := []string{}
	if err := filepath.Walk(c.certRootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return StepResult{}, err
	}
	return StepResult{filesForBackup: files}, nil
}

func (c certsStep) Restore(restoreTo string) error {
	// TODO: add validation?
	return nil
}
