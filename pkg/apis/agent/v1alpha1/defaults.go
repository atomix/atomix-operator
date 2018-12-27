package v1alpha1

func SetDefaults_Cluster(cluster *AtomixCluster) {
	if cluster.Spec.Version == "" {
		cluster.Spec.Version = "3.1.0"
	}
	if cluster.Spec.Controller.Size == 0 {
		cluster.Spec.Controller.Size = 1
	}
	SetDefaults_Storage(&cluster.Spec.Controller.Storage)

	for _, group := range cluster.Spec.PartitionGroups {
		if group.Raft != nil {
			SetDefaults_RaftPartitionGroup(group.Raft)
		} else if group.PrimaryBackup != nil {
			SetDefaults_PrimaryBackupPartitionGroup(group.PrimaryBackup)
		} else if group.Log != nil {
			SetDefaults_LogPartitionGroup(group.Log)
		}
	}
}

// Sets default options on the Raft partition group
func SetDefaults_RaftPartitionGroup(group *RaftPartitionGroup) {
	if group.Size == 0 {
		group.Size = 1
	}
	if group.Partitions == 0 {
		group.Partitions = group.Size
	}
	if group.PartitionSize == 0 {
		group.PartitionSize = 3
	}
	SetDefaults_Storage(&group.Storage)
	SetDefaults_Compaction(&group.Compaction)
}

// Sets default options on the primary-backup partition group
func SetDefaults_PrimaryBackupPartitionGroup(group *PrimaryBackupPartitionGroup) {
	if group.Size == 0 {
		group.Size = 1
	}
	if group.Partitions == 0 {
		group.Partitions = group.Size
	}
	if group.MemberGroupStrategy == "" {
		group.MemberGroupStrategy = NodeAwareMemberGroupStrategy
	}
}

// Sets default options on the log partition group
func SetDefaults_LogPartitionGroup(group *LogPartitionGroup) {
	if group.Size == 0 {
		group.Size = 1
	}
	if group.Partitions == 0 {
		group.Partitions = group.Size
	}
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
