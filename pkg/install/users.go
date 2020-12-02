package install

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
)

// EnsureUser checks if a user exists, and creates it, if it doesn't
// TODO: we should also consider modifying the user, if the user exists, but with wrong settings
func EnsureUser(name string, homeDir string) error {
	shell, err := util.GetExecPath("nologin")
	if err != nil {
		return err
	}

	exists, err := util.CheckIfUserExists(name)
	// User doesn't exist
	if !exists && err == nil {
		// Create the User
		if err := CreateUser(name, homeDir, *shell); err != nil {
			return err
		}
		// User perhaps exists, but cannot be fetched
	} else if err != nil {
		return err
	}
	// verify that user can be fetched, and exists
	_, err = user.Lookup(name)
	if err != nil {
		return err
	}
	return nil
}

// CreateUser creates a system user with either `adduser` or `useradd` command
func CreateUser(userName string, homeDir string, shell string) error {
	var userCmd string
	var userCmdArgs []string

	logrus.Infof("creating user: %s", userName)
	_, err := util.GetExecPath("useradd")
	if err == nil {
		userCmd = "useradd"
		userCmdArgs = []string{`--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName}
	} else {
		userCmd = "adduser"
		userCmdArgs = []string{`--disabled-password`, `--gecos`, `""`, `--home`, homeDir, `--shell`, shell, `--system`, `--no-create-home`, userName}
	}

	cmd := exec.Command(userCmd, userCmdArgs...)
	if err := execCmd(cmd); err != nil {
		return err
	}
	return nil
}

// cmd wrapper
func execCmd(cmd *exec.Cmd) error {
	logrus.Debugf("executing command: %v", quoteCmd(cmd))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command %s: %v", quoteCmd(cmd), err)
	}
	return nil
}

// parse a cmd struct to string
func quoteCmd(cmd *exec.Cmd) string {
	if len(cmd.Args) == 0 {
		return fmt.Sprintf("%q", cmd.Path)
	}

	var q []string
	for _, s := range cmd.Args {
		q = append(q, fmt.Sprintf("%q", s))
	}
	return strings.Join(q, ` `)
}
