/*
 * Copyright 2019 Open Networking Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package chaos

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// exec executes the given command inside the specified container remotely
func exec(kubecli kubernetes.Interface, config *rest.Config, namespace string, podName string, stdinReader io.Reader, container *v1.Container, command ...string) (string, error) {
	logger := log.WithValues("Pod", podName, "Namespace", namespace, "Container", container.Name)

	req := kubecli.CoreV1().RESTClient().Post()
	req = req.Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: container.Name,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
		Stdin:     stdinReader != nil,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())

	if err != nil {
		logger.Error(err, "Creating remote command executor failed")
		return "", err
	}

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	logger.Info("Executing command", "command", command)
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: bufio.NewWriter(&stdout),
		Stderr: bufio.NewWriter(&stderr),
		Stdin:  stdinReader,
		Tty:    false,
	})

	logger.Info(stderr.String())
	logger.Info(stdout.String())

	if err != nil {
		logger.Error(err, "Executing command failed")
		return "", err
	}

	logger.Info("Command succeeded")
	if stderr.Len() > 0 {
		return "", fmt.Errorf("stderr: %v", stderr.String())
	}

	return stdout.String(), nil
}
