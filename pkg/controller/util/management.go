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
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func getManagementResourceName(cluster *v1alpha1.AtomixCluster, resource string) string {
	return cluster.Name + "-" + resource
}

func GetManagementServiceName(cluster *v1alpha1.AtomixCluster) string {
	return getManagementResourceName(cluster, ServiceSuffix)
}

func GetManagementDisruptionBudgetName(cluster *v1alpha1.AtomixCluster) string {
	return getManagementResourceName(cluster, DisruptionBudgetSuffix)
}

func GetManagementInitConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getManagementResourceName(cluster, InitSuffix)
}

func GetManagementSystemConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getManagementResourceName(cluster, ConfigSuffix)
}

func GetManagementStatefulSetName(cluster *v1alpha1.AtomixCluster) string {
	return cluster.Name
}

// NewAppLabels returns a new labels map containing the cluster app
func newManagementLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		AppKey:  cluster.Name,
		TypeKey: ManagementType,
	}
}

// NewManagementDisruptionBudget returns a new pod disruption budget for the Management cluster
func NewManagementDisruptionBudget(cluster *v1alpha1.AtomixCluster) *v1beta1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(int(cluster.Spec.ManagementGroup.Size)/2 + 1)
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetManagementDisruptionBudgetName(cluster),
			Namespace: cluster.Namespace,
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
		},
	}
}

// NewManagementService returns a new headless service for the Atomix cluster
func NewManagementService(cluster *v1alpha1.AtomixCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetManagementServiceName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newManagementLabels(cluster),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: cluster.Name + "-node",
					Port: 5679,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector: map[string]string{
				AppKey: cluster.Name,
			},
		},
	}
}

// NewManagementInitConfigMap returns a new ConfigMap for initializing Atomix clusters
func NewManagementInitConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetManagementInitConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newManagementLabels(cluster),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(cluster),
		},
	}
}

// newInitConfigMapScript returns a new script for generating an Atomix configuration
func newInitConfigMapScript(cluster *v1alpha1.AtomixCluster) string {
	return fmt.Sprintf(`
#!/usr/bin/env bash

HOST=$(hostname -s)
DOMAIN=$(hostname -d)
NODES=$1

function create_config() {
    echo "atomix.service=%s"
    echo "atomix.node.id=$NAME-$ORD"
    echo "atomix.node.host=$NAME-$ORD.$DOMAIN"
    echo "atomix.node.port=5679"
    echo "atomix.replicas=$NODES"
    for (( i=0; i<$NODES; i++ ))
    do
        echo "atomix.members.$i=$NAME-$((i))"
    done
}

if [[ $HOST =~ (.*)-([0-9]+)$ ]]; then
    NAME=${BASH_REMATCH[1]}
    ORD=${BASH_REMATCH[2]}
else
    echo "Failed to parse name and ordinal of Pod"
    exit 1
fi

create_config`, getManagementServiceDnsName(cluster))
}

// Returns the fully qualified DNS name for the management service
func getManagementServiceDnsName(cluster *v1alpha1.AtomixCluster) string {
	return GetManagementServiceName(cluster) + "." + cluster.Namespace + ".svc.cluster.local"
}

// NewManagementSystemConfigMap returns a new ConfigMap for the management cluster
func NewManagementSystemConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetManagementSystemConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newManagementLabels(cluster),
		},
		Data: map[string]string{
			"atomix.conf": newManagementConfig(cluster),
		},
	}
}

// newManagementConfig returns a new Atomix configuration for management nodes
func newManagementConfig(cluster *v1alpha1.AtomixCluster) string {
	return `
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}
managementGroup {
    type: raft
    partitions: 1
    members: ${atomix.members}
    storage.level: disk
}
`
}

// NewManagementStatefulSet returns a StatefulSet for a management cluster
func NewManagementStatefulSet(cluster *v1alpha1.AtomixCluster) (*appsv1.StatefulSet, error) {
	claims, err := newPersistentVolumeClaims(cluster.Spec.ManagementGroup.Storage.ClassName, cluster.Spec.ManagementGroup.Storage.Size)
	if err != nil {
		return nil, err
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetManagementStatefulSetName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newManagementLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetManagementServiceName(cluster),
			Replicas:    &cluster.Spec.ManagementGroup.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newManagementLabels(cluster),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newManagementLabels(cluster),
				},
				Spec: corev1.PodSpec{
					Affinity:       newAffinity(cluster.Name),
					InitContainers: newInitContainers(cluster.Spec.ManagementGroup.Size),
					Containers:     newPersistentContainers(cluster.Spec.Version, cluster.Spec.ManagementGroup.Env, cluster.Spec.ManagementGroup.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetManagementInitConfigMapName(cluster)),
						newUserConfigVolume(GetManagementSystemConfigMapName(cluster)),
						newSystemConfigVolume(),
					},
				},
			},
			VolumeClaimTemplates: claims,
		},
	}, nil
}
