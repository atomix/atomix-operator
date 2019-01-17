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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RaftType          PartitionGroupType = "raft"
	PrimaryBackupType PartitionGroupType = "primary-backup"
	LogType           PartitionGroupType = "log"
)

type PartitionGroupType string

type PartitionGroupSpec struct {
	Cluster       string                       `json:"cluster,omitempty"`
	Version       string                       `json:"version,omitempty"`
	Size          int32                        `json:"size,omitempty"`
	Partitions    int                          `json:"partitions,omitempty"`
	Env           []corev1.EnvVar              `json:"env,omitempty"`
	Resources     corev1.ResourceRequirements  `json:"resources,omitempty"`
	Raft          *RaftPartitionGroup          `json:"raft,omitempty"`
	PrimaryBackup *PrimaryBackupPartitionGroup `json:"primaryBackup,omitempty"`
	Log           *LogPartitionGroup           `json:"log:omitempty"`
}

type PersistentPartitionGroup struct {
	Storage    Storage    `json:"storage,omitempty"`
	Compaction Compaction `json:"compaction,omitempty"`
}

type RaftPartitionGroup struct {
	PersistentPartitionGroup `json:",inline"`
	PartitionSize            int `json:"partitionSize,omitempty"`
}

type PrimaryBackupPartitionGroup struct {
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
	Dynamic          bool    `json:"dynamic,omitempty"`
	FreeDiskBuffer   float32 `json:"freeDiskBuffer,omitempty"`
	FreeMemoryBuffer float32 `json:"freeMemoryBuffer,omitempty"`
}

func GetPartitionGroupType(group *PartitionGroup) (PartitionGroupType, error) {
	switch {
	case group.Spec.Raft != nil:
		return RaftType, nil
	case group.Spec.PrimaryBackup != nil:
		return PrimaryBackupType, nil
	case group.Spec.Log != nil:
		return LogType, nil
	}
	return "", fmt.Errorf("unknown partition group type")
}

// PartitionGroupStatus defines the observed state of AtomixCluster
type PartitionGroupStatus struct {
	// ServiceName is the name of the headless service used to access controller nodes.
	ServiceName string `json:"serviceName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PartitionGroup is the Schema for the partitiongroups API
// +k8s:openapi-gen=true
type PartitionGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PartitionGroupSpec   `json:"spec,omitempty"`
	Status            PartitionGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PartitionGroupList contains a list of PartitionGroup
type PartitionGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PartitionGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PartitionGroup{}, &PartitionGroupList{})
}
