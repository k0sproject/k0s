// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func PodExecPowerShell(client kubernetes.Interface, config *restclient.Config, podName, namespace string, command string) (string, error) {
	cmd := []string{
		"powershell",
		"-Command",
		command,
	}
	return podExecCmdOutput(client, config, podName, namespace, cmd)
}

func PodExecShell(client kubernetes.Interface, config *restclient.Config, podName, namespace string, command string) (string, error) {
	cmd := []string{
		"/bin/sh",
		"-c",
		command,
	}
	return podExecCmdOutput(client, config, podName, namespace, cmd)
}

func podExecCmdOutput(client kubernetes.Interface, config *restclient.Config, podName, namespace string, command []string) (string, error) {
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}
	req.VersionedParams(option, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// PodExecCmdOutput exec command on specific pod and wait the command's output.
func PodExecCmdOutput(client kubernetes.Interface, config *restclient.Config, podName, namespace string, command string) (string, error) {
	cmd := []string{
		"/bin/sh",
		"-c",
		command,
	}

	return podExecCmdOutput(client, config, podName, namespace, cmd)
}
