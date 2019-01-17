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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func getPartitionGroupResourceName(group *v1alpha1.PartitionGroup, resource string) string {
	return fmt.Sprintf("%s-%s", group.Name, resource)
}

func GetPartitionGroupServiceName(group *v1alpha1.PartitionGroup) string {
	return group.Name
}

func GetPartitionGroupDisruptionBudgetName(group *v1alpha1.PartitionGroup) string {
	return getPartitionGroupResourceName(group, DisruptionBudgetSuffix)
}

func GetPartitionGroupInitConfigMapName(group *v1alpha1.PartitionGroup) string {
	return getPartitionGroupResourceName(group, InitSuffix)
}

func GetPartitionGroupSystemConfigMapName(group *v1alpha1.PartitionGroup) string {
	return getPartitionGroupResourceName(group, ConfigSuffix)
}

func GetPartitionGroupStatefulSetName(group *v1alpha1.PartitionGroup) string {
	return group.Name
}

// NewPartitionGroupInitConfigMap returns a new ConfigMap for initializing Atomix clusters
func NewPartitionGroupInitConfigMap(group *v1alpha1.PartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupInitConfigMapName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(types.NamespacedName{group.Namespace, group.Spec.Cluster}),
		},
	}
}

// NewPartitionGroupDisruptionBudget returns a new pod disruption budget for the partition group cluster
func NewPartitionGroupDisruptionBudget(group *v1alpha1.PartitionGroup) *v1beta1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(int(group.Spec.Size)/2 + 1)
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupDisruptionBudgetName(group),
			Namespace: group.Namespace,
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
		},
	}
}

// NewPartitionGroupService returns a new headless service for a partition group
func NewPartitionGroupService(group *v1alpha1.PartitionGroup) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupServiceName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "node",
					Port: 5679,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector:                 newPartitionGroupLabels(group),
		},
	}
}

// newPartitionGroupLabels returns a new labels map containing the cluster app
func newPartitionGroupLabels(group *v1alpha1.PartitionGroup) map[string]string {
	return map[string]string{
		AppKey:     AtomixApp,
		ClusterKey: group.Spec.Cluster,
		TypeKey:    GroupType,
		GroupKey:   group.Name,
	}
}

// NewPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func NewPartitionGroupConfigMap(group *v1alpha1.PartitionGroup) (*corev1.ConfigMap, error) {
	groupType, err := v1alpha1.GetPartitionGroupType(group)
	if err != nil {
		return nil, err
	}
	switch {
	case groupType == v1alpha1.RaftType:
		return newRaftPartitionGroupConfigMap(group), nil
	case groupType == v1alpha1.PrimaryBackupType:
		return newPrimaryBackupPartitionGroupConfigMap(group), nil
	case groupType == v1alpha1.LogType:
		return newLogPartitionGroupConfigMap(group), nil
	}
	return nil, nil
}

// newRaftPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func newRaftPartitionGroupConfigMap(group *v1alpha1.PartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Data: map[string]string{
			"atomix.conf": newRaftPartitionGroupConfig(group),
		},
	}
}

// newRaftPartitionGroupConfig returns a new configuration string for Raft partition group nodes
func newRaftPartitionGroupConfig(group *v1alpha1.PartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}

partitionGroups.%s {
    type: raft
    partitions: %d
    partitionSize: %d
    members: ${atomix.members}
    storage.level: %s
}
`, group.Name, group.Spec.Partitions, group.Spec.Raft.PartitionSize, group.Spec.Raft.Storage.Level)
}

// newPrimaryBackupPartitionGroupConfigMap returns a new ConfigMap for a primary-backup partition group StatefulSet
func newPrimaryBackupPartitionGroupConfigMap(group *v1alpha1.PartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Data: map[string]string{
			"atomix.conf": newPrimaryBackupPartitionGroupConfig(group),
		},
	}
}

// newPrimaryBackupPartitionGroupConfig returns a new configuration string for primary-backup partition group nodes
func newPrimaryBackupPartitionGroupConfig(group *v1alpha1.PartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}

partitionGroups.%s {
    type: primary-backup
    partitions: %d
    memberGroupStrategy: %s
}
`, group.Name, group.Spec.Partitions, group.Spec.PrimaryBackup.MemberGroupStrategy)
}

// newLogPartitionGroupConfigMap returns a new ConfigMap for a log partition group StatefulSet
func newLogPartitionGroupConfigMap(group *v1alpha1.PartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Data: map[string]string{
			"atomix.conf": newLogPartitionGroupConfig(group),
		},
	}
}

// newLogPartitionGroupConfig returns a new configuration string for log partition group nodes
func newLogPartitionGroupConfig(group *v1alpha1.PartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}

partitionGroups.%s {
    type: log
    partitions: %d
    memberGroupStrategy: %s
    storage.level: %s
}
`, group.Name, group.Spec.Partitions, group.Spec.Log.MemberGroupStrategy, group.Spec.Log.Storage.Level)
}

// NewPartitionGroupConfigMap returns a new StatefulSet for a partition group
func NewPartitionGroupStatefulSet(group *v1alpha1.PartitionGroup) (*appsv1.StatefulSet, error) {
	groupType, err := v1alpha1.GetPartitionGroupType(group)
	if err != nil {
		return nil, err
	}
	switch {
	case groupType == v1alpha1.RaftType:
		return newPersistentPartitionGroupStatefulSet(group, &group.Spec.Raft.PersistentPartitionGroup.Storage)
	case groupType == v1alpha1.PrimaryBackupType:
		return newEphemeralPartitionGroupStatefulSet(group)
	case groupType == v1alpha1.LogType:
		return newPersistentPartitionGroupStatefulSet(group, &group.Spec.Log.PersistentPartitionGroup.Storage)
	}
	return nil, nil
}

// newEphemeralPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func newEphemeralPartitionGroupStatefulSet(group *v1alpha1.PartitionGroup) (*appsv1.StatefulSet, error) {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetPartitionGroupServiceName(group),
			Replicas:    &group.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newPartitionGroupLabels(group),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(group),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(group.Spec.Size),
					Containers:     newEphemeralContainers(group.Spec.Version, group.Spec.Env, group.Spec.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(group)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(group)),
						newSystemConfigVolume(),
					},
				},
			},
		},
	}, nil
}

// newPersistentPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func newPersistentPartitionGroupStatefulSet(group *v1alpha1.PartitionGroup, storage *v1alpha1.Storage) (*appsv1.StatefulSet, error) {
	claims, err := newPersistentVolumeClaims(storage.ClassName, storage.Size)
	if err != nil {
		return nil, err
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(group),
			Namespace: group.Namespace,
			Labels:    newPartitionGroupLabels(group),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetPartitionGroupServiceName(group),
			Replicas:    &group.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newPartitionGroupLabels(group),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(group),
				},
				Spec: corev1.PodSpec{
					Affinity:       newAffinity(GetPartitionGroupStatefulSetName(group)),
					InitContainers: newInitContainers(group.Spec.Size),
					Containers:     newPersistentContainers(group.Spec.Version, group.Spec.Env, group.Spec.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(group)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(group)),
						newSystemConfigVolume(),
					},
				},
			},
			VolumeClaimTemplates: claims,
		},
	}, err
}
