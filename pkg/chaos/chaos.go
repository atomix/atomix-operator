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
	"fmt"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func New(client client.Client, scheme *runtime.Scheme, config *rest.Config) *Chaos {
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
	context := c.context.new(c.context.log.WithValues("cluster", cluster.Name, "monkey", config.Name))
	if config.Crash != nil {
		return &Monkey{
			Name:    config.Name,
			cluster: cluster,
			rate:    time.Duration(*config.Crash.RateSeconds * int64(time.Second)),
			handler: &CrashMonkey{
				context: context,
				cluster: cluster,
				config:  config.Crash,
			},
			stopped: make(chan struct{}),
		}
	} else if config.Partition != nil {
		switch config.Partition.PartitionStrategy.Type {
		case v1alpha1.PartitionIsolate:
			return &Monkey{
				Name:    config.Name,
				cluster: cluster,
				rate:    time.Duration(*config.Partition.RateSeconds * int64(time.Second)),
				period:  time.Duration(*config.Partition.PeriodSeconds * int64(time.Second)),
				handler: &PartitionIsolateMonkey{&PartitionMonkey{
					context: context,
					cluster: cluster,
					config:  config.Partition,
				}},
				stopped: make(chan struct{}),
			}
		case v1alpha1.PartitionBridge:
			return &Monkey{
				Name:    config.Name,
				cluster: cluster,
				rate:    time.Duration(*config.Partition.RateSeconds * int64(time.Second)),
				period:  time.Duration(*config.Partition.PeriodSeconds * int64(time.Second)),
				handler: &PartitionBridgeMonkey{&PartitionMonkey{
					context: context,
					cluster: cluster,
					config:  config.Partition,
				}},
				stopped: make(chan struct{}),
			}
		default:
			return &Monkey{Name: config.Name, cluster: cluster, handler: &NilMonkey{}}
		}
	} else {
		return &Monkey{Name: config.Name, cluster: cluster, handler: &NilMonkey{}}
	}
}

type Monkey struct {
	Name    string
	cluster *v1alpha1.AtomixCluster
	Started bool
	handler MonkeyHandler
	stopped chan struct{}
	mu      sync.Mutex
	rate    time.Duration
	period  time.Duration
}

func (m *Monkey) Start() error {
	m.mu.Lock()

	defer utilruntime.HandleCrash()

	// Start the SharedIndexInformer factories to begin populating the SharedIndexInformer caches
	log.Info("Starting monkey", "cluster", m.cluster.Name, "monkey", m.Name)

	if m.period == 0 {
		m.period = 1 * time.Minute
	}

	log.Info("Starting worker", "cluster", m.cluster.Name, "monkey", m.Name)
	go wait.Until(func() {
		var wg wait.Group
		defer wg.Wait()

		stop := make(chan struct{})
		wg.StartWithChannel(stop, m.handler.run)

		t := time.NewTimer(m.period)
		for {
			select {
			case <-m.stopped:
				log.Info("Monkey stopped", "cluster", m.cluster.Name, "monkey", m.Name)
				stop <- struct{}{}
				return
			case <-t.C:
				log.Info("Monkey period expired", "cluster", m.cluster.Name, "monkey", m.Name)
				stop <- struct{}{}
				return
			}
		}
	}, m.rate, m.stopped)

	m.Started = true
	m.mu.Unlock()

	return nil
}

func (m *Monkey) Stop() {
	close(m.stopped)
}

type MonkeyHandler interface {
	run(<-chan struct{})
}

type NilMonkey struct {
	MonkeyHandler
}

func (m *NilMonkey) run(stop <-chan struct{}) {
	<-stop
}
