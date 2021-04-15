/*
Copyright 2021 k0s authors

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
package common

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	ssh "golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
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

func (c *SSHConnection) UploadFile(localPath string, remotePath string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("can't open ssh session: %v",  err)
	}
	stat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("can't stat local file %s: %v", localPath, err)
	}

	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("can't open local file %s: %v", localPath, err)
	}
	defer srcFile.Close()

	dir, filename := path.Split(remotePath)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {

		hostIn, _ := session.StdinPipe()
		defer hostIn.Close()
		fmt.Fprintf(hostIn, "C0664 %d %s\n", stat.Size(), filename)
		io.Copy(hostIn, srcFile)
		fmt.Fprint(hostIn, "\x00")
		wg.Done()
	}()
	if output, err := session.Output(fmt.Sprintf("/usr/bin/scp -t %s", dir)); err != nil {
		return fmt.Errorf("can't execute scp: %s %v", string(output), err)
	}
	wg.Wait()
	return nil
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
