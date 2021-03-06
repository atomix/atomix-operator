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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	AppKey     = "app"
	AtomixApp  = "atomix"
	ClusterKey = "cluster"
	TypeKey    = "type"
	GroupKey   = "group"
)

const (
	ManagementType       = "management"
	GroupType            = "group"
	BenchWorkerType      = "benchmark-worker"
	BenchCoordinatorType = "benchmark-coordinator"
)

const (
	ServiceSuffix          = "service"
	DisruptionBudgetSuffix = "pdb"
	InitSuffix             = "init"
	ConfigSuffix           = "config"
	BenchmarkSuffix        = "bench"
	WorkerSuffix           = "worker"
)

const (
	InitScriptsVolume  = "init-scripts"
	UserConfigVolume   = "user-config"
	SystemConfigVolume = "system-config"
	DataVolume         = "data"
)

func newAffinity(name string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 1,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      AppKey,
									Operator: metav1.LabelSelectorOpIn,
									Values: []string{
										name,
									},
								},
								{
									Key:      TypeKey,
									Operator: metav1.LabelSelectorOpIn,
									Values: []string{
										ManagementType,
									},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
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
				Value: fmt.Sprint(size),
			},
		},
		Command: []string{
			"bash",
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

func newBenchmarkContainers(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) []corev1.Container {
	return []corev1.Container{
		newBenchmarkContainer(version, env, resources),
	}
}

func newBenchmarkContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) corev1.Container {
	args := []string{
		"agent",
		"--config",
		"/etc/atomix/system/atomix.properties",
		"/etc/atomix/user/atomix.conf",
	}
	// TODO: Benchmark version is set to latest as no official release tags exist
	return newContainer(fmt.Sprintf("atomix/atomix-bench:latest"), args, env, resources, newEphemeralVolumeMounts())
}

func newPersistentContainers(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) []corev1.Container {
	return []corev1.Container{
		newPersistentContainer(version, env, resources),
	}
}

func newPersistentContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) corev1.Container {
	args := []string{
		"--config",
		"/etc/atomix/system/atomix.properties",
		"/etc/atomix/user/atomix.conf",
		"--ignore-resources",
		"--data-dir=/var/lib/atomix/data",
		"--log-level=INFO",
		"--file-log-level=OFF",
		"--console-log-level=INFO",
	}
	return newContainer(fmt.Sprintf("atomix/atomix:%s", version), args, env, resources, newPersistentVolumeMounts())
}

func newEphemeralContainers(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) []corev1.Container {
	return []corev1.Container{
		newEphemeralContainer(version, env, resources),
	}
}

func newEphemeralContainer(version string, env []corev1.EnvVar, resources corev1.ResourceRequirements) corev1.Container {
	args := []string{
		"--config",
		"/etc/atomix/system/atomix.properties",
		"/etc/atomix/user/atomix.conf",
		"--ignore-resources",
		"--log-level=INFO",
		"--file-log-level=OFF",
		"--console-log-level=INFO",
	}
	return newContainer(fmt.Sprintf("atomix/atomix:%s", version), args, env, resources, newEphemeralVolumeMounts())
}

func newContainer(image string, args []string, env []corev1.EnvVar, resources corev1.ResourceRequirements, volumeMounts []corev1.VolumeMount) corev1.Container {
	privileged := true
	return corev1.Container{
		Name:            "atomix",
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
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
		Args: args,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
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
	defaultMode := int32(0744)
	return corev1.Volume{
		Name: InitScriptsVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
				DefaultMode: &defaultMode,
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

func newPersistentVolumeClaims(className *string, size string) ([]corev1.PersistentVolumeClaim, error) {
	claim, err := newPersistentVolumeClaim(className, size)
	if err != nil {
		return nil, err
	}
	return []corev1.PersistentVolumeClaim{
		claim,
	}, nil
}

func newPersistentVolumeClaim(className *string, size string) (corev1.PersistentVolumeClaim, error) {
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return corev1.PersistentVolumeClaim{}, err
	}
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: DataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: className,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
	}, nil
}
