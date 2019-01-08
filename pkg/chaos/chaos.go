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

package chaos

import (
	"context"
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sync"
	"time"
)

var log = logf.Log.WithName("chaos_atomix")

var _ manager.Runnable = &Chaos{}

type Chaos struct {
	context Context
	mu      sync.Mutex
	monkeys map[string]*Monkey
	Started bool
	stopped bool
}

func New(client runtimeclient.Client, scheme *runtime.Scheme, config *rest.Config) *Chaos {
	kubecli := kubernetes.NewForConfigOrDie(config)
	context := Context{client, scheme, kubecli, config, log}
	return &Chaos{
		context: context,
		monkeys: make(map[string]*Monkey),
	}
}

func (c *Chaos) Start(stop <-chan struct{}) error {
	c.mu.Lock()

	defer utilruntime.HandleCrash()
	defer c.Stop()

	log.Info("Starting chaos controller")

	c.Started = true
	c.mu.Unlock()

	<-stop

	log.Info("Stopping monkeys")
	for _, monkey := range c.monkeys {
		monkey.Stop()
	}

	return nil
}

func (c *Chaos) Stop() {
	c.stopped = true
}

func (c *Chaos) GetMonkeys() []*Monkey {
	monkeys := []*Monkey{}
	for _, monkey := range c.monkeys {
		monkeys = append(monkeys, monkey)
	}
	return monkeys
}

func getName(cluster *v1alpha1.AtomixCluster, config *v1alpha1.Monkey) string {
	return fmt.Sprintf("%s-%s", cluster.Name, config.Name)
}

func (c *Chaos) GetMonkey(cluster *v1alpha1.AtomixCluster, config *v1alpha1.Monkey) *Monkey {
	name := getName(cluster, config)
	monkey := c.monkeys[name]
	if monkey == nil {
		monkey = c.newMonkey(cluster, config)
		c.monkeys[name] = monkey
	}
	return monkey
}

func (c *Chaos) newMonkey(cluster *v1alpha1.AtomixCluster, config *v1alpha1.Monkey) *Monkey {
	return &Monkey{
		Name:    config.Name,
		cluster: cluster,
		selector: func() ([]v1.Pod, error) {
			return c.selectPods(cluster, config.Selector)
		},
		rate:    time.Duration(*config.RateSeconds * int64(time.Second)),
		period:  time.Duration(*config.PeriodSeconds * int64(time.Second)),
		jitter:  *config.Jitter,
		handler: c.newHandler(cluster, config),
		stopped: make(chan struct{}),
	}
}

func (c *Chaos) newHandler(cluster *v1alpha1.AtomixCluster, config *v1alpha1.Monkey) MonkeyHandler {
	context := c.context.new(c.context.log.WithValues("cluster", cluster.Name, "monkey", config.Name))
	if config.Crash != nil {
		switch config.Crash.CrashStrategy.Type {
		case v1alpha1.CrashContainer:
			return &CrashContainerMonkey{&CrashMonkey{
				context: context,
				cluster: cluster,
				config:  config.Crash,
			}}
		case v1alpha1.CrashPod:
			return &CrashPodMonkey{&CrashMonkey{
				context: context,
				cluster: cluster,
				config:  config.Crash,
			}}
		default:
			return &NilMonkey{}
		}
	} else if config.Partition != nil {
		switch config.Partition.PartitionStrategy.Type {
		case v1alpha1.PartitionIsolate:
			return &PartitionIsolateMonkey{&PartitionMonkey{
				context: context,
				cluster: cluster,
				config:  config.Partition,
			}}
		case v1alpha1.PartitionBridge:
			return &PartitionBridgeMonkey{&PartitionMonkey{
				context: context,
				cluster: cluster,
				config:  config.Partition,
			}}
		default:
			return &NilMonkey{}
		}
	} else if config.Stress != nil {
		return &StressMonkey{
			context: context,
			cluster: cluster,
			config:  config.Stress,
		}
	} else {
		return &NilMonkey{}
	}
}

func (c *Chaos) selectPods(cluster *v1alpha1.AtomixCluster, selector *v1alpha1.MonkeySelector) ([]v1.Pod, error) {
	listOptions := runtimeclient.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: c.newLabelSelector(cluster, selector),
		FieldSelector: c.newFieldSelector(selector),
	}

	// Get a list of pods in the current cluster.
	pods := &v1.PodList{}
	err := c.context.client.List(context.TODO(), &listOptions, pods)
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (c *Chaos) newLabelSelector(cluster *v1alpha1.AtomixCluster, selector *v1alpha1.MonkeySelector) labels.Selector {
	labelSelector := labels.SelectorFromSet(util.NewClusterLabels(cluster))
	if selector != nil {
		if selector.GroupSelector != nil {
			for _, group := range selector.MatchGroups {
				r, err := labels.NewRequirement(util.GroupKey, selection.Equals, []string{group})
				if err == nil {
					labelSelector.Add(*r)
				}
			}
		}

		if selector.LabelSelector != nil {
			for label, value := range selector.MatchLabels {
				r, err := labels.NewRequirement(label, selection.Equals, []string{value})
				if err == nil {
					labelSelector.Add(*r)
				}
			}

			for _, requirement := range selector.MatchExpressions {
				var operator selection.Operator
				switch requirement.Operator {
				case metav1.LabelSelectorOpIn:
					operator = selection.In
				case metav1.LabelSelectorOpNotIn:
					operator = selection.NotIn
				case metav1.LabelSelectorOpExists:
					operator = selection.Exists
				case metav1.LabelSelectorOpDoesNotExist:
					operator = selection.DoesNotExist
				}

				r, err := labels.NewRequirement(requirement.Key, operator, requirement.Values)
				if err == nil {
					labelSelector.Add(*r)
				}
			}
		}
	}
	return labelSelector
}

func (c *Chaos) newFieldSelector(selector *v1alpha1.MonkeySelector) fields.Selector {
	if selector.PodSelector != nil {
		podNames := map[string]string{}
		for _, name := range selector.MatchPods {
			podNames["metadata.name"] = name
		}
		return fields.SelectorFromSet(podNames)
	}
	return nil
}

type Monkey struct {
	Name     string
	cluster  *v1alpha1.AtomixCluster
	selector func() ([]v1.Pod, error)
	Started  bool
	handler  MonkeyHandler
	stopped  chan struct{}
	mu       sync.Mutex
	rate     time.Duration
	period   time.Duration
	jitter   float64
}

func (m *Monkey) Start() error {
	m.mu.Lock()

	defer utilruntime.HandleCrash()

	logger := log.WithValues("cluster", m.cluster.Name, "monkey", m.Name)

	// Start the SharedIndexInformer factories to begin populating the SharedIndexInformer caches
	logger.Info("Starting monkey")

	if m.period == 0 {
		m.period = 1 * time.Minute
	}

	logger.Info("Starting worker")

	go func() {
		// wait.Until will immediately trigger the monkey, so we need to wait for the configured rate first.
		t := time.NewTimer(m.rate)
		<-t.C

		// Run the monkey every rate for the configured period until the monkey is stopped.
		wait.JitterUntil(func() {
			var wg wait.Group
			defer wg.Wait()

			stop := make(chan struct{})
			wg.StartWithChannel(stop, func(stop <-chan struct{}) {
				pods, err := m.selector()
				if err != nil {
					logger.Error(err, "Failed to select pods")
				} else if len(pods) == 0 {
					logger.Info("No pods selected")
				} else {
					m.handler.run(pods, stop)
				}
			})

			t := time.NewTimer(m.period)
			for {
				select {
				case <-m.stopped:
					logger.Info("Monkey stopped")
					stop <- struct{}{}
					return
				case <-t.C:
					logger.Info("Monkey period expired")
					stop <- struct{}{}
					return
				}
			}
		}, m.rate, m.jitter, true, m.stopped)
	}()

	m.Started = true
	m.mu.Unlock()

	return nil
}

func (m *Monkey) Stop() {
	close(m.stopped)
}

type MonkeyHandler interface {
	run([]v1.Pod, <-chan struct{})
}

type NilMonkey struct{}

func (m *NilMonkey) run(pods []v1.Pod, stop <-chan struct{}) {
	<-stop
}
