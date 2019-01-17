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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AtomixBenchmarkSpec
type AtomixBenchmarkSpec struct {
	Cluster   string                      `json:"cluster,omitempty"`
	Version   string                      `json:"version,omitempty"`
	Workers   int32                       `json:"workers,omitempty"`
	Env       []corev1.EnvVar             `json:"env,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// AtomixBenchmarkStatus defines the observed state of AtomixCluster
type AtomixBenchmarkStatus struct {
	// ServiceName is the name of the headless service used to access controller nodes.
	ServiceName string `json:"serviceName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AtomixBenchmark is the Schema for the AtomixBenchmarks API
// +k8s:openapi-gen=true
type AtomixBenchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AtomixBenchmarkSpec   `json:"spec,omitempty"`
	Status            AtomixBenchmarkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AtomixBenchmarkList contains a list of BenchmarkController
type AtomixBenchmarkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AtomixBenchmark `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AtomixBenchmark{}, &AtomixBenchmarkList{})
}
