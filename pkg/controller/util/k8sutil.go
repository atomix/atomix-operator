package util

import (
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	AppKey   string = "app"
	GroupKey string = "group"
)

const (
	ServiceSuffix string = "service"
	InitSuffix    string = "init"
	ConfigSuffix  string = "config"
)

const (
	InitScriptsVolume  string = "init-scripts"
	UserConfigVolume   string = "user-config"
	SystemConfigVolume string = "system-config"
	DataVolume         string = "data"
)

func getControllerResourceName(cluster *v1alpha1.AtomixCluster, resource string) string {
	return cluster.Name + "-" + resource
}

func GetControllerServiceName(cluster *v1alpha1.AtomixCluster) string {
	return getControllerResourceName(cluster, ServiceSuffix)
}

func GetControllerInitConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getControllerResourceName(cluster, InitSuffix)
}

func GetControllerSystemConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getControllerResourceName(cluster, ConfigSuffix)
}

func GetControllerStatefulSetName(cluster *v1alpha1.AtomixCluster) string {
	return cluster.Name
}

// NewAppLabels returns a new labels map containing the cluster app
func newControllerLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		AppKey: cluster.Name,
	}
}

// NewControllerService returns a new headless service for the Atomix cluster
func NewControllerService(cluster *v1alpha1.AtomixCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-hs",
			Namespace: cluster.Namespace,
			Labels:    newControllerLabels(cluster),
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
			ClusterIP:                nil,
			Selector:                 newControllerLabels(cluster),
		},
	}
}

// NewControllerInitConfigMap returns a new ConfigMap for initializing Atomix clusters
func NewControllerInitConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetControllerInitConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newControllerLabels(cluster),
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

create_config`, cluster.Name)
}

// NewControllerSystemConfigMap returns a new ConfigMap for the controller cluster
func NewControllerSystemConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetControllerSystemConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newControllerLabels(cluster),
		},
		Data: map[string]string{
			"atomix.conf": newControllerConfig(cluster),
		},
	}
}

// newControllerConfig returns a new Atomix configuration for controller nodes
func newControllerConfig(cluster *v1alpha1.AtomixCluster) string {
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

// NewControllerStatefulSet returns a StatefulSet for a a controller
func NewControllerStatefulSet(cluster *v1alpha1.AtomixCluster) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetControllerStatefulSetName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newControllerLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newControllerLabels(cluster),
				},
				Spec: corev1.PodSpec{
					Affinity:       newAffinity(cluster.Name),
					InitContainers: newInitContainers(cluster.Spec.Controller.Size),
					Containers:     newPersistentContainers(cluster.Spec.Version, cluster.Spec.Controller.Env, cluster.Spec.Controller.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetControllerInitConfigMapName(cluster)),
						newUserConfigVolume(GetControllerSystemConfigMapName(cluster)),
						newSystemConfigVolume(),
					},
				},
			},
			VolumeClaimTemplates: newPersistentVolumeClaims(cluster.Spec.Controller.Storage.Size),
		},
	}
}

func newAffinity(name string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      AppKey,
								Operator: metav1.LabelSelectorOpIn,
								Values: []string{
									name,
								},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
}

func newInitContainers(size int32) []corev1.Container {
	return []corev1.Container{
		newInitContainer(size),
	}
}

func newInitContainer(size int32) corev1.Container {
	return corev1.Container{
		Name:  "configure",
		Image: "ubuntu:16.04",
		Env: []corev1.EnvVar{
			{
				Name:  "ATOMIX_NODES",
				Value: string(size),
			},
		},
		Command: []string{
			"sh",
			"-c",
			"/scripts/create_config.sh $ATOMIX_NODES > /config/atomix.properties",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      InitScriptsVolume,
				MountPath: "/scripts",
			},
			{
				Name:      SystemConfigVolume,
				MountPath: "/config",
			},
		},
	}
}

func newPersistentContainers(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) []corev1.Container {
	return []corev1.Container{
		newPersistentContainer(version, env, resources),
	}
}

func newPersistentContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) corev1.Container {
	return newContainer(version, env, resources, newPersistentVolumeMounts())
}

func newEphemeralContainers(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) []corev1.Container {
	return []corev1.Container{
		newEphemeralContainer(version, env, resources),
	}
}

func newEphemeralContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) corev1.Container {
	return newContainer(version, env, resources, newEphemeralVolumeMounts())
}

func newContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements, volumeMounts []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            "atomix",
		Image:           fmt.Sprintf("atomix/atomix:%s", version),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             env,
		Resources:       resources,
		Ports: []corev1.ContainerPort{
			{
				Name:          "client",
				ContainerPort: 5678,
			},
			{
				Name:          "server",
				ContainerPort: 5679,
			},
		},
		Args: []string{
			"--config",
			"/etc/atomix/system/atomix.properties",
			"/etc/atomix/user/atomix.conf",
			"--ignore-resources",
			"--data-dir=/var/lib/atomix/data",
			"--log-level=debug",
			"--file-log-level=off",
			"--console-log-level=debug",
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/status",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      10,
			FailureThreshold:    6,
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/status",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      10,
		},
		VolumeMounts: volumeMounts,
	}
}

func newPersistentVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		newDataVolumeMount(),
		newUserConfigVolumeMount(),
		newSystemConfigVolumeMount(),
	}
}

func newEphemeralVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		newUserConfigVolumeMount(),
		newSystemConfigVolumeMount(),
	}
}

func newDataVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      DataVolume,
		MountPath: "/var/lib/atomix",
	}
}

func newUserConfigVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      UserConfigVolume,
		MountPath: "/etc/atomix/user",
	}
}

func newSystemConfigVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      SystemConfigVolume,
		MountPath: "/etc/atomix/system",
	}
}

func newInitScriptsVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: InitScriptsVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
			},
		},
	}
}

func newUserConfigVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: UserConfigVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
			},
		},
	}
}

func newSystemConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name: SystemConfigVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func newPersistentVolumeClaims(size string) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		newPersistentVolumeClaim(size),
	}
}

func newPersistentVolumeClaim(size string) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: DataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.Quantity{Format: resource.Format(size)},
				},
			},
		},
	}
}

func getPartitionGroupBaseName(cluster *v1alpha1.AtomixCluster, name string) string {
	return cluster.Name + "-" + name
}

func getPartitionGroupResourceName(cluster *v1alpha1.AtomixCluster, name string, resource string) string {
	return getPartitionGroupBaseName(cluster, name) + "-" + resource
}

func GetPartitionGroupServiceName(cluster *v1alpha1.AtomixCluster, group string) string {
	return getPartitionGroupResourceName(cluster, group, ServiceSuffix)
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
			Labels:    newControllerLabels(cluster),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(cluster),
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
			ClusterIP:                nil,
			Selector:                 newPartitionGroupLabels(cluster, group),
		},
	}
}

// newPartitionGroupLabels returns a new labels map containing the cluster app
func newPartitionGroupLabels(cluster *v1alpha1.AtomixCluster, group string) map[string]string {
	return map[string]string{
		AppKey:   cluster.Name,
		GroupKey: group,
	}
}

// NewRaftPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func NewRaftPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.RaftPartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Data: map[string]string{
			"atomix.conf": NewRaftPartitionGroupConfig(cluster, name, group),
		},
	}
}

// NewRaftPartitionGroupConfig returns a new configuration string for Raft partition group nodes
func NewRaftPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.RaftPartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: %s,
    }
}

partitionGroups.%s {
    type: raft
    partitions: %d
    partitionSize: %d
    members: ${atomix.members}
    storage.level: %s
}
`, cluster.Name, name, group.Partitions, group.PartitionSize, group.Storage.Level)
}

// NewPrimaryBackupPartitionGroupConfigMap returns a new ConfigMap for a primary-backup partition group StatefulSet
func NewPrimaryBackupPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PrimaryBackupPartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Data: map[string]string{
			"atomix.conf": NewPrimaryBackupPartitionGroupConfig(cluster, name, group),
		},
	}
}

// NewPrimaryBackupPartitionGroupConfig returns a new configuration string for primary-backup partition group nodes
func NewPrimaryBackupPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PrimaryBackupPartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: %s,
    }
}

partitionGroups.%s {
    type: primary-backup
    partitions: %d
    memberGroupStrategy: %s
}
`, cluster.Name, name, group.Partitions, group.MemberGroupStrategy)
}

// NewLogPartitionGroupConfigMap returns a new ConfigMap for a log partition group StatefulSet
func NewLogPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.LogPartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupSystemConfigMapName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Data: map[string]string{
			"atomix.conf": NewLogPartitionGroupConfig(cluster, name, group),
		},
	}
}

// NewLogPartitionGroupConfig returns a new configuration string for log partition group nodes
func NewLogPartitionGroupConfig(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.LogPartitionGroup) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: %s,
    }
}

partitionGroups.%s {
    type: log
    partitions: %d
    memberGroupStrategy: %s
    storage.level: %s
}
`, cluster.Name, name, group.Partitions, group.MemberGroupStrategy, group.Storage.Level)
}

// NewEphemeralPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func NewEphemeralPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroup) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(cluster, name),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(group.Size),
					Containers:     newEphemeralContainers(cluster.Spec.Version, group.Env, group.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(cluster, name)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(cluster, name)),
						newSystemConfigVolume(),
					},
				},
			},
		},
	}
}

// NewPersistentPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func NewPersistentPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PersistentPartitionGroup) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPartitionGroupStatefulSetName(cluster, name),
			Namespace: cluster.Namespace,
			Labels:    newPartitionGroupLabels(cluster, name),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newPartitionGroupLabels(cluster, name),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(group.Size),
					Containers:     newPersistentContainers(cluster.Spec.Version, group.Env, group.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetPartitionGroupInitConfigMapName(cluster, name)),
						newUserConfigVolume(GetPartitionGroupSystemConfigMapName(cluster, name)),
						newSystemConfigVolume(),
					},
				},
			},
			VolumeClaimTemplates: newPersistentVolumeClaims(group.Storage.Size),
		},
	}
}
