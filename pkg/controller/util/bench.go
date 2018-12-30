package util

import (
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// newBenchmarkLabels returns a new labels for benchmark nodes
func newBenchmarkCoordinatorLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		AppKey:  cluster.Name,
		TypeKey: BenchCoordinatorType,
	}
}

func getBenchmarkCoordinatorResourceName(cluster *v1alpha1.AtomixCluster, resource string) string {
	return cluster.Name + "-" + BenchmarkSuffix + "-" + resource
}

func GetBenchmarkCoordinatorServiceName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkCoordinatorResourceName(cluster, ServiceSuffix)
}

func GetBenchmarkCoordinatorIngressName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkCoordinatorResourceName(cluster, IngressSuffix)
}

func GetBenchmarkCoordinatorInitConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkCoordinatorResourceName(cluster, InitSuffix)
}

func GetBenchmarkCoordinatorSystemConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkCoordinatorResourceName(cluster, ConfigSuffix)
}

func GetBenchmarkCoordinatorPodName(cluster *v1alpha1.AtomixCluster) string {
	return cluster.Name + "-" + BenchmarkSuffix
}

func NewBenchmarkCoordinatorInitConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkCoordinatorInitConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkCoordinatorLabels(cluster),
		},
		Data: map[string]string{
			"create_config.sh": newCoordinatorInitConfigMapScript(cluster),
		},
	}
}

// newCoordinatorInitConfigMapScript returns a new script for generating an Atomix configuration
func newCoordinatorInitConfigMapScript(cluster *v1alpha1.AtomixCluster) string {
	return fmt.Sprintf(`
#!/usr/bin/env bash

HOST=$(hostname -s)

function create_config() {
    echo "atomix.service=%s"
    echo "atomix.node.id=$HOST"
    echo "atomix.node.host=$HOST"
    echo "atomix.node.port=5679"
}

create_config`, getManagementServiceDnsName(cluster))
}

func NewBenchmarkCoordinatorSystemConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkCoordinatorSystemConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkCoordinatorLabels(cluster),
		},
		Data: map[string]string{
			"atomix.conf": newBenchmarkCoordinatorConfig(cluster),
		},
	}
}

// newBenchmarkCoordinatorConfig returns a new configuration string for a benchmark coordinator node
func newBenchmarkCoordinatorConfig(cluster *v1alpha1.AtomixCluster) string {
	return fmt.Sprintf(`
cluster {
    discovery {
        type: dns
        service: ${atomix.service},
    }
}`)
}

func NewBenchmarkCoordinatorPod(cluster *v1alpha1.AtomixCluster) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkCoordinatorPodName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkCoordinatorLabels(cluster),
		},
		Spec: corev1.PodSpec{
			InitContainers: newInitContainers(1),
			Containers:     newEphemeralContainers(cluster.Spec.Version, cluster.Spec.Benchmark.Env, cluster.Spec.Benchmark.Resources),
			Volumes: []corev1.Volume{
				newInitScriptsVolume(GetBenchmarkCoordinatorInitConfigMapName(cluster)),
				newUserConfigVolume(GetBenchmarkCoordinatorSystemConfigMapName(cluster)),
				newSystemConfigVolume(),
			},
		},
	}
}

func NewBenchmarkCoordinatorService(cluster *v1alpha1.AtomixCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkCoordinatorServiceName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkCoordinatorLabels(cluster),
			Annotations: map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: cluster.Name + "-api",
					Port: 5678,
				},
				{
					Name: cluster.Name + "-node",
					Port: 5679,
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
			Selector:                 newBenchmarkCoordinatorLabels(cluster),
		},
	}
}

func NewBenchmarkCoordinatorIngress(cluster *v1alpha1.AtomixCluster) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkCoordinatorIngressName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkCoordinatorLabels(cluster),
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"nginx.ingress.kubernetes.io/ssl-redirect": "false",
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: cluster.Spec.Benchmark.Ingress.Path,
									Backend: v1beta1.IngressBackend{
										ServiceName: GetBenchmarkCoordinatorServiceName(cluster),
										ServicePort: intstr.FromInt(5678),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// newBenchmarkLabels returns a new labels for benchmark nodes
func newBenchmarkWorkerLabels(cluster *v1alpha1.AtomixCluster) map[string]string {
	return map[string]string{
		AppKey:  cluster.Name,
		TypeKey: BenchWorkerType,
	}
}

func getBenchmarkWorkerResourceName(cluster *v1alpha1.AtomixCluster, resource string) string {
	return cluster.Name + "-" + WorkerSuffix + "-" + resource
}

func GetBenchmarkWorkerServiceName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkWorkerResourceName(cluster, ServiceSuffix)
}

func GetBenchmarkWorkerInitConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkWorkerResourceName(cluster, InitSuffix)
}

func GetBenchmarkWorkerSystemConfigMapName(cluster *v1alpha1.AtomixCluster) string {
	return getBenchmarkWorkerResourceName(cluster, ConfigSuffix)
}

func GetBenchmarkWorkerStatefulSetName(cluster *v1alpha1.AtomixCluster) string {
	return cluster.Name + "-" + WorkerSuffix
}

func NewBenchmarkWorkerInitConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerInitConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkWorkerLabels(cluster),
		},
		Data: map[string]string{
			"create_config.sh": newInitConfigMapScript(cluster),
		},
	}
}

func NewBenchmarkWorkerSystemConfigMap(cluster *v1alpha1.AtomixCluster) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerSystemConfigMapName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkWorkerLabels(cluster),
		},
		Data: map[string]string{
			"atomix.conf": newBenchmarkWorkerConfig(cluster),
		},
	}
}

// newBenchmarkWorkerConfig returns a new configuration string for a benchmark coordinator node
func newBenchmarkWorkerConfig(cluster *v1alpha1.AtomixCluster) string {
	return fmt.Sprintf(`
cluster {
    node: ${atomix.node}

    discovery {
        type: dns
        service: ${atomix.service},
    }
}`)
}

func NewBenchmarkWorkerStatefulSet(cluster *v1alpha1.AtomixCluster) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerStatefulSetName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkWorkerLabels(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetBenchmarkWorkerServiceName(cluster),
			Replicas:    &cluster.Spec.Benchmark.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: newBenchmarkWorkerLabels(cluster),
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: newBenchmarkWorkerLabels(cluster),
				},
				Spec: corev1.PodSpec{
					InitContainers: newInitContainers(cluster.Spec.Benchmark.Size),
					Containers:     newBenchmarkContainers(cluster.Spec.Version, cluster.Spec.Benchmark.Env, cluster.Spec.Benchmark.Resources),
					Volumes: []corev1.Volume{
						newInitScriptsVolume(GetBenchmarkWorkerInitConfigMapName(cluster)),
						newUserConfigVolume(GetBenchmarkWorkerSystemConfigMapName(cluster)),
						newSystemConfigVolume(),
					},
				},
			},
		},
	}
}

func NewBenchmarkWorkerService(cluster *v1alpha1.AtomixCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetBenchmarkWorkerServiceName(cluster),
			Namespace: cluster.Namespace,
			Labels:    newBenchmarkWorkerLabels(cluster),
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
			Selector:                 newBenchmarkWorkerLabels(cluster),
		},
	}
}