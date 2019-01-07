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
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StressMonkey struct {
	MonkeyHandler
	context Context
	cluster *v1alpha1.AtomixCluster
	config  *v1alpha1.StressMonkey
}

func (m *StressMonkey) run(stop <-chan struct{}) {
	switch m.config.StressStrategy.Type {
	case v1alpha1.StressRandom:
		m.stressRandom(stop)
	case v1alpha1.StressAll:
		m.stressAll(stop)
	}
}

func (m *StressMonkey) stressRandom(stop <-chan struct{}) {
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
		m.context.log.Info("No pods to stress")
		return
	}

	// Choose a random node to kill.
	pod := pods.Items[rand.Intn(len(pods.Items))]

	m.stressPod(pod)

	<-stop

	m.destressPod(pod)
}

func (m *StressMonkey) stressAll(stop <-chan struct{}) {
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
		m.context.log.Info("No pods to stress")
		return
	}

	for _, pod := range pods.Items {
		m.stressPod(pod)
	}

	<-stop

	for _, pod := range pods.Items {
		m.destressPod(pod)
	}
}

func (m *StressMonkey) stressPod(pod v1.Pod) {
	if m.config.IO != nil || m.config.CPU != nil || m.config.Memory != nil || m.config.HDD != nil {
		// Build a 'stress' command and arguments.
		command := []string{}
		command = append(command, "stress")
		if m.config.IO != nil {
			command = append(command, "--io")
			command = append(command, string(*m.config.IO.Workers))
		}
		if m.config.CPU != nil {
			command = append(command, "--cpu")
			command = append(command, string(*m.config.CPU.Workers))
		}
		if m.config.Memory != nil {
			command = append(command, "--vm")
			command = append(command, string(*m.config.Memory.Workers))
		}
		if m.config.HDD != nil {
			command = append(command, "--hdd")
			command = append(command, string(*m.config.HDD.Workers))
		}

		container := pod.Spec.Containers[0]
		log.Info("Stressing container", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)

		// Execute the command inside the Atomix container.
		_, err := m.context.exec(pod, &container, command...)
		if err != nil {
			m.context.log.Error(err, "Failed to stress container", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)
		}
	}

	// If network stress is configured, build a 'tc' command to inject latency into the network.
	if m.config.Network != nil {
		command := []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay"}
		command = append(command, fmt.Sprintf("%sms", string(*m.config.Network.LatencyMilliseconds)))
		command = append(command, fmt.Sprintf("%sms", string(int(*m.config.Network.Jitter*float64(*m.config.Network.LatencyMilliseconds)))))
		command = append(command, fmt.Sprintf("%f", *m.config.Network.Correlation))
		command = append(command, string(*m.config.Network.Distribution))

		container := pod.Spec.Containers[0]
		log.Info("Slowing container network", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)

		// Execute the command inside the Atomix container.
		_, err := m.context.exec(pod, &container, command...)
		if err != nil {
			m.context.log.Error(err, "Failed to slow container network", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)
		}
	}
}

func (m *StressMonkey) destressPod(pod v1.Pod) {
	// If the 'stress' utility options are enabled, kill the 'stress' process.
	if m.config.IO != nil || m.config.CPU != nil || m.config.Memory != nil || m.config.HDD != nil {
		command := []string{"pkill", "-f", "stress"}

		container := pod.Spec.Containers[0]
		log.Info("Destressing container", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)

		_, err := m.context.exec(pod, &container, command...)
		if err != nil {
			m.context.log.Error(err, "Failed to destress container", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)
		}
	}

	// If the network stress options are enabled, delete the 'tc' rule injecting latency.
	if m.config.Network != nil {
		command := []string{"tc", "qdisc", "del", "dev", "eth0", "root"}

		container := pod.Spec.Containers[0]
		log.Info("Restoring container network", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)

		_, err := m.context.exec(pod, &container, command...)
		if err != nil {
			m.context.log.Error(err, "Failed to restore container network", "pod", pod.Name, "namespace", pod.Namespace, "container", container.Name)
		}
	}
}
