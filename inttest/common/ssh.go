package common

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	ssh "golang.org/x/crypto/ssh"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

// SSHConnection describes an SSH connection
type SSHConnection struct {
	Address string
	User    string
	Port    int
	KeyPath string

	client *ssh.Client
}

// Disconnect closes the SSH connection
func (c *SSHConnection) Disconnect() {
	c.client.Close()
}

// Connect opens the SSH connection
func (c *SSHConnection) Connect() error {
	key, err := loadExternalFile(c.KeyPath)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User:            c.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	address := fmt.Sprintf("%s:%d", c.Address, c.Port)

	sshAgentSock := os.Getenv("SSH_AUTH_SOCK")
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil && sshAgentSock == "" {
		return err
	}
	if err == nil {
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return err
	}
	c.client = client

	return nil
}

// ExecCmd executes a command on the host
func (c *SSHConnection) ExecCmd(cmd string, stdin string, streamStdout bool) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	if stdin == "" {
		// FIXME not requesting a pty for commands with stdin input for now,
		// as it appears the pipe doesn't get closed with stdinpipe.Close()
		modes := ssh.TerminalModes{}
		err = session.RequestPty("xterm", 80, 40, modes)
		if err != nil {
			return err
		}
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return err
	}

	if err := session.Start(cmd); err != nil {
		return err
	}

	if stdin != "" {
		if len(stdin) > 256 {
			log.Debugf("%s: writing %d bytes to command stdin", c.Address, len(stdin))
		} else {
			log.Debugf("%s: writing %d bytes to command stdin: %s", c.Address, len(stdin), stdin)
		}

		go func() {
			defer stdinPipe.Close()
			io.WriteString(stdinPipe, stdin)
		}()
	}

	multiReader := io.MultiReader(stdout, stderr)
	outputScanner := bufio.NewScanner(multiReader)

	for outputScanner.Scan() {
		if streamStdout {
			log.Infof("%s:  %s", c.Address, outputScanner.Text())
		}
	}
	if err := outputScanner.Err(); err != nil {
		log.Errorf("%s:  %s", c.Address, err.Error())
	}

	return session.Wait()
}

// ExecWithOutput execs a command on the host and returns its output
func (c *SSHConnection) ExecWithOutput(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return trimOutput(output), err
	}

	return trimOutput(output), nil
}

func trimOutput(output []byte) string {
	if len(output) == 0 {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func loadExternalFile(path string) ([]byte, error) {
	realpath, err := homedir.Expand(path)
	if err != nil {
		return []byte{}, err
	}

	filedata, err := ioutil.ReadFile(realpath)
	if err != nil {
		return []byte{}, err
	}
	return filedata, nil
}
