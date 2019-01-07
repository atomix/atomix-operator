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
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CrashMonkey struct {
	MonkeyHandler
	context Context
	cluster *v1alpha1.AtomixCluster
	config  *v1alpha1.CrashMonkey
}

func (m *CrashMonkey) run(stop <-chan struct{}) {
	selector := labels.SelectorFromSet(util.NewClusterLabels(m.cluster))

	listOptions := client.ListOptions{
		Namespace:     m.cluster.Namespace,
		LabelSelector: selector,
	}

	// Get a list of pods in the current cluster.
	pods := &v1.PodList{}
	err := m.context.client.List(context.TODO(), &listOptions, pods)
	if err != nil {
		m.context.log.Error(err, "Failed to list pods")
	}

	// If there are no pods listed, exit the monkey.
	if len(pods.Items) == 0 {
		m.context.log.Info("No pods to kill")
		return
	}

	// Choose a random node to kill.
	pod := pods.Items[rand.Intn(len(pods.Items))]
	container := pod.Spec.Containers[0]

	log.Info("Killing pod", "pod", pod.Name, "namespace", pod.Namespace)

	// Kill the node by killing the Java process running inside the container. This forces health checks to be used
	// to recover the node.
	_, err = m.context.exec(pod, &container, "bash", "-c", "kill -9 $(ps -ef | grep AtomixAgent | grep -v grep | cut -c10-15 | tr -d ' ')")

	if err != nil {
		m.context.log.Error(err, "Failed to kill pod", "pod", pod.Name, "namespace", pod.Namespace)
	}

	<-stop
}
