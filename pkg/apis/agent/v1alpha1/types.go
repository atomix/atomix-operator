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
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Controller Controller `json:"controller,omitempty"`
	Version string `json:"nodes,omitempty"`
	PartitionGroups map[string]PartitionGroupSpec `json:"partitionGroups,omitempty"`
}

// Controller node configuration
type Controller struct {
	Size int32 `json:"size,omitempty"`
	Env []v1.EnvVar `json:"env,omitempty"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	Storage Storage `json:"storage,omitempty"`
}

const (
	RaftType PartitionGroupType = "raft"
	PrimaryBackupType PartitionGroupType = "primary-backup"
	LogType PartitionGroupType = "log"
)

type PartitionGroupType string

type PartitionGroupSpec struct {
	Raft *RaftPartitionGroup
	PrimaryBackup *PrimaryBackupPartitionGroup
	Log *LogPartitionGroup
}

type PartitionGroup struct {
	Size int32 `json:"size,omitempty""`
	Partitions int `json:"partitions,omitempty""`
	Env []v1.EnvVar `json:"env,omitempty"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}

type PersistentPartitionGroup struct {
	PartitionGroup `json:",inline"`
	Storage Storage `json:"storage,omitempty"`
	Compaction Compaction `json:"compaction,omitempty"`
}

type RaftPartitionGroup struct {
	PersistentPartitionGroup `json:",inline"`
	PartitionSize int `json:"partitionSize,omitempty"`
}

type PrimaryBackupPartitionGroup struct {
	PartitionGroup `json:",inline"`
	MemberGroupStrategy MemberGroupStrategy `json:"memberGroupStrategy,omitempty"`
}

type LogPartitionGroup struct {
	PersistentPartitionGroup `json:",inline"`
	MemberGroupStrategy MemberGroupStrategy `json:"memberGroupStrategy,omitempty"`
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
	DiskStorage StorageLevel = "disk"
	MappedStorage StorageLevel = "mapped"
)

type Storage struct {
	Size string `json:"size,omitempty"`
	SegmentSize string `json:"segmentSize,omitempty"`
	EntrySize string `json:"entrySize,omitempty"`
	Level StorageLevel `json:"level,omitempty"`
	FlushOnCommit bool `json:"flushOnCommit,omitempty"`
}

type Compaction struct {
	Dynamic bool `json:"dynamic,omitempty""`
	FreeDiskBuffer float32 `json:"freeDiskBuffer,omitempty"`
	FreeMemoryBuffer float32 `json:"freeMemoryBuffer,omitempty"`
}

// ClusterStatus defines the observed state of AtomixCluster
type ClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file

	// ServiceName is the name of the headless service used to access controller nodes.
	ServiceName string `json:"serviceName,omitempty"`
}

func GetPartitionGroupType(group *PartitionGroupSpec) (PartitionGroupType, error) {
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
	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
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
