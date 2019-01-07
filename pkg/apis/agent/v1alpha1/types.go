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
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of AtomixCluster
type ClusterSpec struct {
	ManagementGroup ManagementGroup      `json:"managementGroup,omitempty"`
	Version         string               `json:"version,omitempty"`
	PartitionGroups []PartitionGroupSpec `json:"partitionGroups"`
	Benchmark       *Benchmark           `json:"benchmark,omitempty"`
	Chaos           Chaos                `json:"chaos,omitempty"`
}

// Chaos defines the chaos monkey configuration
type Chaos struct {
	Monkeys []Monkey `json:"monkeys,omitempty"`
}

// Chaos monkey configuration
type Monkey struct {
	Name      string           `json:"name,omitempty"`
	Crash     *CrashMonkey     `json:"crash,omitempty"`
	Partition *PartitionMonkey `json:"partition,omitempty"`
}

// Chaos monkey that crashes nodes
type CrashMonkey struct {
	RateSeconds *int64 `json:"rateSeconds,omitempty"`
}

// Chaos monkey that partitions nodes
type PartitionMonkey struct {
	RateSeconds       *int64            `json:"rateSeconds,omitempty"`
	PeriodSeconds     *int64            `json:"periodSeconds,omitempty"`
	PartitionStrategy PartitionStrategy `json:"partitionStrategy,omitempty"`
}

// Partition strategy.
type PartitionStrategy struct {
	Type PartitionStrategyType `json:"type,omitempty"`
}

type PartitionStrategyType string

const (
	PartitionIsolate PartitionStrategyType = "Isolate"
	PartitionBridge  PartitionStrategyType = "Bridge"
)

// Management group configuration
type ManagementGroup struct {
	Size      int32                   `json:"size,omitempty"`
	Env       []v1.EnvVar             `json:"env,omitempty"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	Storage   Storage                 `json:"storage,omitempty"`
}

const (
	RaftType          PartitionGroupType = "raft"
	PrimaryBackupType PartitionGroupType = "primary-backup"
	LogType           PartitionGroupType = "log"
)

// Benchmark cluster configuration
type Benchmark struct {
	Size      int32                   `json:"size,omitempty"`
	Env       []v1.EnvVar             `json:"env,omitempty"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}

type PartitionGroupType string

type PartitionGroupSpec struct {
	Name          string                       `json:"name,omitempty"`
	Size          int32                        `json:"size,omitempty""`
	Partitions    int                          `json:"partitions,omitempty""`
	Env           []v1.EnvVar                  `json:"env,omitempty"`
	Resources     v1.ResourceRequirements      `json:"resources,omitempty"`
	Raft          *RaftPartitionGroup          `json:"raft,omitempty"`
	PrimaryBackup *PrimaryBackupPartitionGroup `json:"primaryBackup,omitempty"`
	Log           *LogPartitionGroup           `json:"log:omitempty"`
}

type PartitionGroup struct{}

type PersistentPartitionGroup struct {
	PartitionGroup `json:",inline"`
	Storage        Storage    `json:"storage,omitempty"`
	Compaction     Compaction `json:"compaction,omitempty"`
}

type RaftPartitionGroup struct {
	PersistentPartitionGroup `json:",inline"`
	PartitionSize            int `json:"partitionSize,omitempty"`
}

type PrimaryBackupPartitionGroup struct {
	PartitionGroup      `json:",inline"`
	MemberGroupStrategy MemberGroupStrategy `json:"memberGroupStrategy,omitempty"`
}

type LogPartitionGroup struct {
	PersistentPartitionGroup `json:",inline"`
	MemberGroupStrategy      MemberGroupStrategy `json:"memberGroupStrategy,omitempty"`
}

// MemberGroupStrategy describes the way partition members are balanced among member groups.
type MemberGroupStrategy string

const (
	NodeAwareMemberGroupStrategy MemberGroupStrategy = "node_aware"
	RackAwareMemberGroupStrategy MemberGroupStrategy = "rack_aware"
	ZoneAwareMemberGroupStrategy MemberGroupStrategy = "zone_aware"
)

// StorageLevel describes the storage level of commit logs.
type StorageLevel string

const (
	DiskStorage   StorageLevel = "disk"
	MappedStorage StorageLevel = "mapped"
)

type Storage struct {
	Size          string       `json:"size,omitempty"`
	ClassName     *string      `json:"className,omitempty"`
	SegmentSize   string       `json:"segmentSize,omitempty"`
	EntrySize     string       `json:"entrySize,omitempty"`
	Level         StorageLevel `json:"level,omitempty"`
	FlushOnCommit bool         `json:"flushOnCommit,omitempty"`
}

type Compaction struct {
	Dynamic          bool    `json:"dynamic,omitempty""`
	FreeDiskBuffer   float32 `json:"freeDiskBuffer,omitempty"`
	FreeMemoryBuffer float32 `json:"freeMemoryBuffer,omitempty"`
}

// ClusterStatus defines the observed state of AtomixCluster
type ClusterStatus struct {
	// ServiceName is the name of the headless service used to access controller nodes.
	ServiceName string `json:"serviceName,omitempty"`
}

func GetPartitionGroupType(group *PartitionGroupSpec) (PartitionGroupType, error) {
	if group.Name == "" {
		return "", fmt.Errorf("unnamed partition group")
	}
	switch {
	case group.Raft != nil:
		return RaftType, nil
	case group.PrimaryBackup != nil:
		return PrimaryBackupType, nil
	case group.Log != nil:
		return LogType, nil
	}
	return "", fmt.Errorf("unknown partition group type")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AtomixCluster is the Schema for the atomixclusters API
// +k8s:openapi-gen=true
type AtomixCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterSpec   `json:"spec,omitempty"`
	Status            ClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AtomixClusterList contains a list of AtomixCluster
type AtomixClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AtomixCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AtomixCluster{}, &AtomixClusterList{})
}
