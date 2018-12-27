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


// NewAppLabels returns a new labels map containing the cluster app
func NewAppLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		"app": cluster.Name,
	}
}

// NewClusterService returns a new headless service for the Atomix cluster
func NewClusterService(cluster *v1alpha1.AtomixCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster.Name + "-hs",
			Namespace: cluster.Namespace,
			Labels: NewAppLabels(cluster),
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
			ClusterIP: nil,
			Selector: map[string]string{
				"app": cluster.Name,
			},
		},
	}
}

// NewInitConfigMap returns a new ConfigMap for initializing Atomix clusters
func NewInitConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		Data: map[string]string {
			"create_config.sh": NewInitConfigMapScript(cluster),
		},
	}
}

// NewInitConfigMapScript returns a new script for generating an Atomix configuration
func NewInitConfigMapScript(cluster *v1alpha1.AtomixCluster) string {
	return fmt.Sprintf(`
#!/usr/bin/env bash

HOST=$(hostname -s)
DOMAIN=$(hostname -d)

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

// NewControllerConfigMap returns a new ConfigMap for the controller cluster
func NewControllerConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		Data: map[string]string{
			"atomix.conf": NewControllerConfig(cluster),
		},
	}
}

// NewControllerConfig returns a new Atomix configuration for controller nodes
func NewControllerConfig(cluster *v1alpha1.AtomixCluster) string {
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
			Name: cluster.Name,
			Namespace: cluster.Namespace,
			Labels: NewAppLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: NewAppLabels(cluster),
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											metav1.LabelSelectorRequirement{
												Key: "app",
												Operator: metav1.LabelSelectorOpIn,
												Values: []string{
													cluster.Name,
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name: "atomix",
							Image: fmt.Sprintf("atomix/atomix:%s", cluster.Spec.Version),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: cluster.Spec.Controller.Env,
							Resources: cluster.Spec.Controller.Resources,
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: 5678},
								{Name: "server", ContainerPort: 5679},
							},
							Args: []string{
								"--config",
								"/etc/atomix/atomix.conf",
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
								TimeoutSeconds: 10,
								FailureThreshold: 6,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/status",
										Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds: 10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "data",
									MountPath: "/var/lib/atomix",
								},
								{
									Name: "user-config",
									MountPath: "/etc/atomix/user",
								},
								{
									Name: "system-config",
									MountPath: "/etc/atomix/system",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "init-scripts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cluster.Name + "-init",
									},
								},
							},
						},
						{
							Name: "user-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cluster.Name + "-config",
									},
								},
							},
						},
						{
							Name: "system-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.Quantity{Format: resource.Format(cluster.Spec.Controller.Storage.Size)},
							},
						},
					},
				},
			},
		},
	}
}

// NewRaftPartitionGroupConfigMap returns a new ConfigMap for a Raft partition group StatefulSet
func NewRaftPartitionGroupConfigMap(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.RaftPartitionGroup) *corev1.ConfigMap {
	return &corev1.ConfigMap{
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
    storage.level: %s
}
`, cluster.Name, name, group.Partitions, group.Storage.Level)
}

// NewEphemeralPartitionGroupStatefulSet returns a new StatefulSet for a persistent partition group
func NewEphemeralPartitionGroupStatefulSet(cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroup) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster.Name + "" + name,
			Namespace: cluster.Namespace,
			Labels: NewAppLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: NewAppLabels(cluster),
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											metav1.LabelSelectorRequirement{
												Key: "app",
												Operator: metav1.LabelSelectorOpIn,
												Values: []string{
													cluster.Name,
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name: "atomix",
							Image: fmt.Sprintf("atomix/atomix:%s", cluster.Spec.Version),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: group.Env,
							Resources: group.Resources,
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: 5678},
								{Name: "server", ContainerPort: 5679},
							},
							Args: []string{
								"--config",
								"/etc/atomix/atomix.conf",
								"--ignore-resources",
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
								TimeoutSeconds: 10,
								FailureThreshold: 6,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/status",
										Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds: 10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/atomix"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "config", VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cluster.Name + "-config",
								},
							},
						}},
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
			Name: cluster.Name + "" + name,
			Namespace: cluster.Namespace,
			Labels: NewAppLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &cluster.Spec.Controller.Size,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: NewAppLabels(cluster),
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											metav1.LabelSelectorRequirement{
												Key: "app",
												Operator: metav1.LabelSelectorOpIn,
												Values: []string{
													cluster.Name,
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name: "atomix",
							Image: fmt.Sprintf("atomix/atomix:%s", cluster.Spec.Version),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: group.Env,
							Resources: group.Resources,
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: 5678},
								{Name: "server", ContainerPort: 5679},
							},
							Args: []string{
								"--config",
								"/etc/atomix/atomix.conf",
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
								TimeoutSeconds: 10,
								FailureThreshold: 6,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/status",
										Port: intstr.IntOrString{Type: intstr.Int, IntVal: 5678},
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds: 10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/atomix"},
								{Name: "config", MountPath: "/etc/atomix"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "config", VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cluster.Name + "-config",
								},
							},
						}},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": resource.Quantity{Format: resource.Format(group.Storage.Size)},
							},
						},
					},
				},
			},
		},
	}
}
