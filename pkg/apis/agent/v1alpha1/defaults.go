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

func SetDefaults_Cluster(cluster *AtomixCluster) {
	if cluster.Spec.Version == "" {
		cluster.Spec.Version = "3.1.0"
	}
	if cluster.Spec.ManagementGroup.Size == 0 {
		cluster.Spec.ManagementGroup.Size = 1
	}
	SetDefaults_Storage(&cluster.Spec.ManagementGroup.Storage)

	for _, group := range cluster.Spec.PartitionGroups {
		SetDefaults_PartitionGroup(&group)
	}

	SetDefaults_Chaos(&cluster.Spec.Chaos)
}

func SetDefaults_Chaos(chaos *Chaos) {
	minute := int64(60)
	for _, monkey := range chaos.Monkeys {
		if monkey.Crash != nil {
			if monkey.Crash.RateSeconds == nil {
				monkey.Crash.RateSeconds = &minute
			}
		} else if monkey.Partition != nil {
			if monkey.Partition.RateSeconds == nil {
				monkey.Partition.RateSeconds = &minute
			}
			if monkey.Partition.PeriodSeconds == nil {
				monkey.Partition.PeriodSeconds = &minute
			}
			if monkey.Partition.PartitionStrategy.Type == "" {
				monkey.Partition.PartitionStrategy.Type = PartitionIsolate
			}
		}
	}
}

func SetDefaults_PartitionGroup(group *PartitionGroupSpec) {
	if group.Size == 0 {
		group.Size = 1
	}
	if group.Partitions == 0 {
		group.Partitions = int(group.Size)
	}
	if group.Raft != nil {
		SetDefaults_RaftPartitionGroup(group.Raft)
	} else if group.PrimaryBackup != nil {
		SetDefaults_PrimaryBackupPartitionGroup(group.PrimaryBackup)
	} else if group.Log != nil {
		SetDefaults_LogPartitionGroup(group.Log)
	}
}

// Sets default options on the Raft partition group
func SetDefaults_RaftPartitionGroup(group *RaftPartitionGroup) {
	if group.PartitionSize == 0 {
		group.PartitionSize = 3
	}
	SetDefaults_Storage(&group.Storage)
	SetDefaults_Compaction(&group.Compaction)
}

// Sets default options on the primary-backup partition group
func SetDefaults_PrimaryBackupPartitionGroup(group *PrimaryBackupPartitionGroup) {
	if group.MemberGroupStrategy == "" {
		group.MemberGroupStrategy = NodeAwareMemberGroupStrategy
	}
}

// Sets default options on the log partition group
func SetDefaults_LogPartitionGroup(group *LogPartitionGroup) {
	if group.MemberGroupStrategy == "" {
		group.MemberGroupStrategy = NodeAwareMemberGroupStrategy
	}
	SetDefaults_Storage(&group.Storage)
	SetDefaults_Compaction(&group.Compaction)
}

func SetDefaults_Storage(storage *Storage) {
	if storage.Size == "" {
		storage.Size = "2G"
	}
	if storage.SegmentSize == "" {
		storage.SegmentSize = "64M"
	}
	if storage.EntrySize == "" {
		storage.EntrySize = "1K"
	}
	if storage.Level == "" {
		storage.Level = MappedStorage
	}
}

func SetDefaults_Compaction(compaction *Compaction) {
	if compaction.FreeDiskBuffer == 0 {
		compaction.FreeDiskBuffer = .2
	}
	if compaction.FreeMemoryBuffer == 0 {
		compaction.FreeMemoryBuffer = .2
	}
}
