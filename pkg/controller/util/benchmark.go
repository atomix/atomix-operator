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

package util

import (
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// newBenchmarkControllerLabels returns a new labels for benchmark nodes
func newBenchmarkControllerLabels(benchmark *v1alpha1.AtomixBenchmark) map[string]string {
	return map[string]string{
		AppKey:  AtomixApp,
		ClusterKey: benchmark.Name,
		TypeKey: BenchCoordinatorType,
	}
}

func getBenchmarkControllerResourceName(benchmark *v1alpha1.AtomixBenchmark, resource string) string {
	return fmt.Sprintf("%s-%s", benchmark.Name, resource)
}

func GetBenchmarkControllerServiceName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkControllerResourceName(benchmark, ServiceSuffix)
}

func GetBenchmarkControllerInitConfigMapName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkControllerResourceName(benchmark, InitSuffix)
}

func GetBenchmarkControllerSystemConfigMapName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkControllerResourceName(benchmark, ConfigSuffix)
}

func GetBenchmarkControllerPodName(benchmark *v1alpha1.AtomixBenchmark) string {
	return benchmark.Name
}

func NewBenchmarkControllerInitConfigMap(benchmark *v1alpha1.AtomixBenchmark) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkControllerInitConfigMapName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkControllerLabels(benchmark),
		},
		Data: map[string]string{
			"create_config.sh": newBenchmarkControllerInitConfigMapScript(benchmark),
		},
	}
}

// newBenchmarkControllerInitConfigMapScript returns a new script for generating an Atomix configuration
func newBenchmarkControllerInitConfigMapScript(benchmark *v1alpha1.AtomixBenchmark) string {
	return fmt.Sprintf(`
#!/usr/bin/env bash

HOST=$(hostname -s)

function create_config() {
    echo "atomix.service=%s"
    echo "atomix.node.id=$HOST"
    echo "atomix.node.host=%s"
    echo "atomix.node.port=5679"
}

create_config`, getManagementServiceDnsName(types.NamespacedName{benchmark.Namespace, benchmark.Spec.Cluster}), GetBenchmarkControllerServiceName(benchmark))
}

func NewBenchmarkControllerSystemConfigMap(benchmark *v1alpha1.AtomixBenchmark) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkControllerSystemConfigMapName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkControllerLabels(benchmark),
		},
		Data: map[string]string{
			"atomix.conf": newBenchmarkControllerConfig(benchmark),
		},
	}
}

// newBenchmarkControllerConfig returns a new configuration string for a benchmark coordinator node
func newBenchmarkControllerConfig(benchmark *v1alpha1.AtomixBenchmark) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}`)
}

func NewBenchmarkControllerPod(benchmark *v1alpha1.AtomixBenchmark) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkControllerPodName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkControllerLabels(benchmark),
		},
		Spec: corev1.PodSpec{
			InitContainers: newInitContainers(1),
			Containers:     newBenchmarkContainers(benchmark.Spec.Version, benchmark.Spec.Env, benchmark.Spec.Resources),
			Volumes: []corev1.Volume{
				newInitScriptsVolume(GetBenchmarkControllerInitConfigMapName(benchmark)),
				newUserConfigVolume(GetBenchmarkControllerSystemConfigMapName(benchmark)),
				newSystemConfigVolume(),
			},
		},
	}
}

func NewBenchmarkControllerService(benchmark *v1alpha1.AtomixBenchmark) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkControllerServiceName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkControllerLabels(benchmark),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: benchmark.Name + "-api",
					Port: 5678,
				},
				{
					Name: benchmark.Name + "-node",
					Port: 5679,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector:                 newBenchmarkControllerLabels(benchmark),
		},
	}
}

// newBenchmarkLabels returns a new labels for benchmark nodes
func newBenchmarkWorkerLabels(benchmark *v1alpha1.AtomixBenchmark) map[string]string {
	return map[string]string{
		AppKey:  AtomixApp,
		TypeKey: BenchWorkerType,
	}
}

func getBenchmarkWorkerResourceName(benchmark *v1alpha1.AtomixBenchmark, resource string) string {
	return fmt.Sprintf("%s-%s", benchmark.Name, resource)
}

func GetBenchmarkWorkerServiceName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkWorkerResourceName(benchmark, ServiceSuffix)
}

func GetBenchmarkWorkerInitConfigMapName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkWorkerResourceName(benchmark, InitSuffix)
}

func GetBenchmarkWorkerSystemConfigMapName(benchmark *v1alpha1.AtomixBenchmark) string {
	return getBenchmarkWorkerResourceName(benchmark, ConfigSuffix)
}

func GetBenchmarkWorkerStatefulSetName(benchmark *v1alpha1.AtomixBenchmark) string {
	return benchmark.Name
}

func NewBenchmarkWorkerInitConfigMap(benchmark *v1alpha1.AtomixBenchmark) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerInitConfigMapName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkWorkerLabels(benchmark),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(types.NamespacedName{benchmark.Namespace, benchmark.Name}),
		},
	}
}

func NewBenchmarkWorkerSystemConfigMap(benchmark *v1alpha1.AtomixBenchmark) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerSystemConfigMapName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkWorkerLabels(benchmark),
		},
		Data: map[string]string{
			"atomix.conf": newBenchmarkWorkerConfig(benchmark),
		},
	}
}

// newBenchmarkWorkerConfig returns a new configuration string for a benchmark coordinator node
func newBenchmarkWorkerConfig(benchmark *v1alpha1.AtomixBenchmark) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}`)
}

func NewBenchmarkWorkerStatefulSet(benchmark *v1alpha1.AtomixBenchmark) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerStatefulSetName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkWorkerLabels(benchmark),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetBenchmarkWorkerServiceName(benchmark),
			Replicas:    &benchmark.Spec.Workers,
			Selector: &metav1.LabelSelector{
				MatchLabels: newBenchmarkWorkerLabels(benchmark),
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newBenchmarkWorkerLabels(benchmark),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(benchmark.Spec.Workers),
					Containers:     newBenchmarkContainers(benchmark.Spec.Version, benchmark.Spec.Env, benchmark.Spec.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetBenchmarkWorkerInitConfigMapName(benchmark)),
						newUserConfigVolume(GetBenchmarkWorkerSystemConfigMapName(benchmark)),
						newSystemConfigVolume(),
					},
				},
			},
		},
	}
}

func NewBenchmarkWorkerService(benchmark *v1alpha1.AtomixBenchmark) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerServiceName(benchmark),
			Namespace: benchmark.Namespace,
			Labels:    newBenchmarkWorkerLabels(benchmark),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: benchmark.Name + "-node",
					Port: 5679,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector:                 newBenchmarkWorkerLabels(benchmark),
		},
	}
}
