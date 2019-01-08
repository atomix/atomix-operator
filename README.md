# Atomix Operator

This project provides a Kubernetes Operator for [Atomix][Atomix].

The Atomix operator facilitates the deployment of Atomix clusters, the maintenance of
partition groups, fault injection and performance testing on k8s:
* [Setup](#setup)
* [Operation](#operation)
* [Labels](#labels)
* [Benchmarking](#benchmarking)
* [Fault injection and stress testing](#fault-injection-and-stress-testing)

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

## Fault injection and stress testing

The Atomix operator provides a full suite of tools for chaos testing, injecting
a variety of failures into the nodes and in the Atomix network. Each cluster spec
can support an arbitrary number of _chaos monkey_ configurations, and each monkey
plays a specific role in injecting failures into the cluster. Monkeys are configured
through the `chaos` field of the cluster spec:

```yaml
spec:
  chaos:
    monkeys:
    - name: crash
      rateSeconds: 60
      jitter: .5
      crash:
        crashStrategy:
          type: Container
    - name: partition-isolate
      rateSeconds: 600
      periodSeconds: 120
      selector:
        matchGroups:
        - raft
      partition:
        partitionStrategy:
          type: Isolate
    - name: stress-cpu
      rateSeconds: 300
      periodSeconds: 300
      stress:
        stressStrategy:
          type: All
        cpu:
          workers: 2
```

Each monkey configuration supports both a _rate_ and _period_ for which the crash occurs:
* `rateSeconds` - the number of seconds to wait between monkey runs
* `periodSeconds` - the number of seconds for which to run a monkey, e.g. the amount of
time for which to partition the network or stress a node
* `jitter` - the amount of jitter to apply to the rate

Additionally, specific sets of pods can be selected using pod names, group names, or
labels specified in the configured `selector`:
* `matchPods` - a list of pod names on which to match
* `matchGroups` - a list of Atomix partition group names on which to match
* `matchLabels` - a map of label names and values on which to match pods
* `matchExpressions` - label match expressions on which to match pods

Selector options can be added on a per-monkey basis:

```yaml
selector:
  matchPods:
  - pod-1
  - pod-2
  - pod-3
  matchGroups:
  - raft
  - data
  matchLabels:
    group: raft
  matchExpressions:
  - key: group
    operator: In
    values:
    - raft
    - data
```

Each monkey type has a custom configuration provided by a named field for the
monkey type:
* [`crash`](#crash-monkey)
* [`partition`](#partition-monkey)
* [`stress`](#stress-monkey)

### Crash monkey

The crash monkey can be used to inject node crashes into the cluster. To configure a
crash monkey, use the `crash` configuration:

```yaml
monkeys:
- name: crash
  rateSeconds: 60
  jitter: .5
  crash:
    crashStrategy:
      type: Container
```

The `crash` configuration supports a `crashStrategy` with the following options:
* `Container` - kills the Atomix process running inside the container
* `Pod` - deletes the `Pod` using the Kubernetes API

### Partition monkey

The partition monkey can be used to cut off network communication between a set of
nodes in the Atomix cluster. To configure a partition monkey, use the `partition`
configuration:

```yaml
monkeys:
- name: partition-isolate
  rateSeconds: 600
  periodSeconds: 120
  partition:
    partitionStrategy:
      type: Isolate
```

The `partition` configuration supports a `partitionStrategy` with the following options:
* `Isolate` - isolates a single random node in the cluster from all other nodes
* `Bridge` - splits the cluster into two halves with a single bridge node able to
communicate with each half (for testing consensus)

### Stress monkey

The stress monkey uses a variety of tools to simulate stress on nodes and on the 
network. To configure a stress monkey, use the `stress` configuration:

```yaml
monkeys:
- name: stress-cpu
  rateSeconds: 300
  periodSeconds: 300
  stress:
    stressStrategy:
      type: All
    cpu:
      workers: 2
```

The `stress` configuration supports a `stressStrategy` with the following options:
* `Random` - applies stress options to a random pod
* `All` - applies stress options to all pods in the cluster

The stress monkey supports a variety of types of stress using the
[stress](https://linux.die.net/man/1/stress) tool:
* `cpu` - spawns `cpu.workers` workers spinning on `sqrt()`
* `io` - spawns `io.workers` workers spinning on `sync()`
* `memory` - spawns `memory.workers` workers spinning on `malloc()`/`free()`
* `hdd` - spawns `hdd.workers` workers spinning on `write()`/`unlink()`

```yaml
monkeys:
- name: stress-all
  rateSeconds: 300
  periodSeconds: 300
  stress:
    stressStrategy:
      type: Random
    cpu:
      workers: 2
    io:
      workers: 2
    memory:
      workers: 4
    hdd:
      workers: 1
```

Additionally, network latency can be injected using the stress monkey via
[traffic control](http://man7.org/linux/man-pages/man8/tc-netem.8.html) by providing
a `network` stress configuration:
* `latencyMilliseconds` - the amount of latency to inject in milliseconds
* `jitter` - the jitter to apply to the latency
* `correlation` - the correlation to apply to the latency
* `distribution` - the delay distribution, either `normal`, `pareto`, or `paretonormal`

```yaml
monkeys:
- name: stress-network
  rateSeconds: 300
  periodSeconds: 60
  stress:
    stressStrategy:
      type: All
    network:
      latencyMilliseconds: 500
      jitter: .5
      correlation: .25
```

[Atomix]: https://atomix.io
