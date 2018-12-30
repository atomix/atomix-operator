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

func getPartitionGroupBaseName(cluster *v1alpha1.AtomixCluster, name string) string {
	return cluster.Name + "-" + name
}

func getPartitionGroupResourceName(cluster *v1alpha1.AtomixCluster, name string, resource string) string {
	return getPartitionGroupBaseName(cluster, name) + "-" + resource
}

func GetPartitionGroupServiceName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupResourceName(cluster, group, ServiceSuffix)
}

func GetPartitionGroupDisruptionBudgetName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupResourceName(cluster, group, DisruptionBudgetSuffix)
}

func GetPartitionGroupInitConfigMapName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupResourceName(cluster, group, InitSuffix)
}

func GetPartitionGroupSystemConfigMapName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupResourceName(cluster, group, ConfigSuffix)
}

func GetPartitionGroupStatefulSetName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupBaseName(cluster, group)
}

// NewPartitionGroupInitConfigMap returns a new ConfigMap for initializing Atomix clusters
func NewPartitionGroupInitConfigMap(cluster *v1alpha1.AtomixCluster, name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupInitConfigMapName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(cluster),
		},
	}
}

// NewPartitionGroupDisruptionBudget returns a new pod disruption budget for the partition group cluster
func NewPartitionGroupDisruptionBudget(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) *v1beta1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(int(group.Size)/2 + 1)
	return &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupDisruptionBudgetName(cluster, group.Name),
			Namespace: cluster.Namespace,
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
		},
	}
}

// NewPartitionGroupService returns a new headless service for a partition group
func NewPartitionGroupService(cluster *v1alpha1.AtomixCluster, group string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupServiceName(cluster, group),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, group),
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
			Selector:                 newPartitionGroupLabels(cluster, group),
		},
	}
}

// NewPartitionGroupClusterLabels returns labels for identifying partition group clusters
func NewPartitionGroupClusterLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		AppKey:  cluster.Name,
		TypeKey: GroupType,
	}
}

// newPartitionGroupLabels returns a new labels map containing the cluster app
func newPartitionGroupLabels(cluster *v1alpha1.AtomixCluster, group string) map[string]string {
	return map[string]string{
		AppKey:   cluster.Name,
		TypeKey:  GroupType,
		GroupKey: group,
	}
}

// NewPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func NewPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) (*corev1.ConfigMap, error) {
	groupType, err := v1alpha1.GetPartitionGroupType(group)
	if err != nil {
		return nil, err
	}
	switch {
	case groupType == v1alpha1.RaftType:
		return newRaftPartitionGroupConfigMap(cluster, group), nil
	case groupType == v1alpha1.PrimaryBackupType:
		return newPrimaryBackupPartitionGroupConfigMap(cluster, group), nil
	case groupType == v1alpha1.LogType:
		return newLogPartitionGroupConfigMap(cluster, group), nil
	}
	return nil, nil
}

// newRaftPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func newRaftPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, group.Name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, group.Name),
		},
		Data: map[string]string{
			"atomix.conf": newRaftPartitionGroupConfig(cluster, group.Name, group),
		},
	}
}

// newRaftPartitionGroupConfig returns a new configuration string for Raft partition group nodes
func newRaftPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroupSpec) string {
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
`, name, group.Partitions, group.Raft.PartitionSize, group.Raft.Storage.Level)
}

// newPrimaryBackupPartitionGroupConfigMap returns a new ConfigMap for a primary-backup partition group StatefulSet
func newPrimaryBackupPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, group.Name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, group.Name),
		},
		Data: map[string]string{
			"atomix.conf": newPrimaryBackupPartitionGroupConfig(cluster, group.Name, group),
		},
	}
}

// newPrimaryBackupPartitionGroupConfig returns a new configuration string for primary-backup partition group nodes
func newPrimaryBackupPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroupSpec) string {
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
`, name, group.Partitions, group.PrimaryBackup.MemberGroupStrategy)
}

// newLogPartitionGroupConfigMap returns a new ConfigMap for a log partition group StatefulSet
func newLogPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, group.Name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, group.Name),
		},
		Data: map[string]string{
			"atomix.conf": newLogPartitionGroupConfig(cluster, group.Name, group),
		},
	}
}

// newLogPartitionGroupConfig returns a new configuration string for log partition group nodes
func newLogPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroupSpec) string {
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
`, name, group.Partitions, group.Log.MemberGroupStrategy, group.Log.Storage.Level)
}

// NewPartitionGroupConfigMap returns a new StatefulSet for a partition group
func NewPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, group *v1alpha1.PartitionGroupSpec) (*appsv1.StatefulSet, error) {
	groupType, err := v1alpha1.GetPartitionGroupType(group)
	if err != nil {
		return nil, err
	}
	switch {
	case groupType == v1alpha1.RaftType:
		return newPersistentPartitionGroupStatefulSet(cluster, group, &group.Raft.PersistentPartitionGroup)
	case groupType == v1alpha1.PrimaryBackupType:
		return newEphemeralPartitionGroupStatefulSet(cluster, group, &group.PrimaryBackup.PartitionGroup)
	case groupType == v1alpha1.LogType:
		return newPersistentPartitionGroupStatefulSet(cluster, group, &group.Log.PersistentPartitionGroup)
	}
	return nil, nil
}

// newEphemeralPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func newEphemeralPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, spec *v1alpha1.PartitionGroupSpec, group *v1alpha1.PartitionGroup) (*appsv1.StatefulSet, error) {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(cluster, spec.Name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, spec.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetPartitionGroupServiceName(cluster, spec.Name),
			Replicas:    &spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newPartitionGroupLabels(cluster, spec.Name),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(cluster, spec.Name),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(spec.Size),
					Containers:     newEphemeralContainers(cluster.Spec.Version, spec.Env, spec.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(cluster, spec.Name)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(cluster, spec.Name)),
						newSystemConfigVolume(),
					},
				},
			},
		},
	}, nil
}

// newPersistentPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func newPersistentPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, spec *v1alpha1.PartitionGroupSpec, group *v1alpha1.PersistentPartitionGroup) (*appsv1.StatefulSet, error) {
	claims, err := newPersistentVolumeClaims(group.Storage.ClassName, group.Storage.Size)
	if err != nil {
		return nil, err
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(cluster, spec.Name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, spec.Name),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetPartitionGroupServiceName(cluster, spec.Name),
			Replicas:    &spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newPartitionGroupLabels(cluster, spec.Name),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(cluster, spec.Name),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(spec.Size),
					Containers:     newPersistentContainers(cluster.Spec.Version, spec.Env, spec.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(cluster, spec.Name)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(cluster, spec.Name)),
						newSystemConfigVolume(),
					},
				},
			},
			VolumeClaimTemplates: claims,
		},
	}, err
}
