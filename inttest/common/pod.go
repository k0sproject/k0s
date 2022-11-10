package common

import (
	"bytes"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodExecCmdOutput exec command on specific pod and wait the command's output.
func PodExecCmdOutput(client kubernetes.Interface, config *restclient.Config, podName, namespace string, command string) (string, error) {
	cmd := []string{
		"/bin/sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
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

	var b bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &b,
		Stderr: &b,
	})
	if err != nil {
		return b.String(), err
	}

	return b.String(), nil
}
