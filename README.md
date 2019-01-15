# Atomix Operator

This project provides a Kubernetes Operator for [Atomix][Atomix].

The Atomix operator facilitates the deployment of Atomix clusters, the maintenance of
partition groups, and performance testing on k8s:
* [Setup](#setup)
* [Operation](#operation)
* [Labels](#labels)
* [Benchmarking](#benchmarking)

## Setup

Before running the operator, register the Atomix CRD:

```
$ kubectl create -f deploy/crds/atomixcluster_crd.yaml
```

Setup RBAC and deploy the operator:

```
$ kubectl create -f deploy/service_account.yaml
$ kubectl create -f deploy/role.yaml
$ kubectl create -f deploy/role_binding.yaml
$ kubectl create -f deploy/operator.yaml
```

## Operation

The operator provides an `AtomixCluster` resource which can be used to deploy Atomix
clusters with any number of partition groups. The operator will deploy a single
`StatefulSet` for the configured `managementGroup` and a `StatefulSet` for each of the
configured `partitionGroups`. Both the management group and the partition groups and
their protocols can be configured in the Kubernetes manifest.

```
apiVersion: agent.atomix.io/v1alpha1
kind: AtomixCluster
metadata:
  name: example-atomixcluster
spec:
  version: 3.1.0
  managementGroup:
    size: 1
    storage:
      className: fast-disks
  partitionGroups:
    - name: raft
      size: 3
      partitions: 3
      raft:
        storage:
          className: fast-disks
```

```
$ kubectl create -f deploy/crds/atomixcluster_cr.yaml
```

Client nodes can join the Atomix cluster through the `AtomixCluster` service:

```
cluster.discovery {
  type: dns
  service: example-atomixcluster-service.default.svc.cluster.local
}
```

Once a client has joined the cluster, it will automatically discover all running
partition groups.

Partition groups can be added to or removed from an existing cluster by patching the
resource using the `merge` strategy:

```
$ kubectl patch atomixcluster example-atomixcluster --patch "$(cat deploy/crds/atomixcluster_cr_raft.yaml)" --type=merge
```

When a partition group is added, the operator will create a new `StatefulSet` and service
for the partition group, and when a partition group is removed the associated `StatefulSet`
and service will be removed.

### Partition group types

Each partition group supports the following options:
* `name` - the name of the group
* `size` - the number of nodes in the group
* `partitions` - the number of partitions in the group
* `env` - environment variables for the partition group nodes
* `resources` - container resources for the partition group nodes

Partition group types are configured by providing a protocol-specific configuration in the
partition group specification:
* `raft` - Raft partition group configuration
* `primaryBackup` - Primary backup group configuration
* `log` - Distributed log group configuration

```
  partitionGroups:
    - name: raft
      size: 3
      partitions: 3
      raft:
        storage:
          className: fast-disks
    - name: primary-backup
      size: 2
      partitions: 7
      primaryBackup: {}
    - name: log
      size: 3
      partitions: 3
      log:
        storage:
          className: fast-disks
```

### Labels

The management group resources are labeled with the following labels:
* `app`: `{clusterName}`
* `type`: `managment`

Partition group resources are labeled with the following labels:
* `app`: `{clusterName}`
* `type`: `group`
* `group`: `{partitionGroupName}`

Benchmark worker resources are labeled with the following labels:
* `app`: `{clusterName}`
* `type`: `benchmark-worker`

Benchmark coordinator resources are labeled with the following labels:
* `app`: `{clusterName}`
* `type`: `benchmark-coordinator`

### Benchmarking

The operator also supports [atomix-bench](https://github.com/atomix/atomix-bench)
workers when a `benchmark` spec is provided:

```
spec:
  benchmark:
    size: 3
```

```
$ kubectl patch atomixcluster example-atomixcluster --patch "$(cat deploy/crds/atomixcluster_cr_bench.yaml)" --type=merge
```

When a `benchmark` spec is provided, the operator will deploy `n` benchmark worker nodes
and a benchmark coordinator with an ingress for managing benchmarks.

[Atomix]: https://atomix.io
