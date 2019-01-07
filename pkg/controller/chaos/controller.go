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
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/chaos"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller_atomix")

func New(client client.Client, scheme *runtime.Scheme, chaos *chaos.Chaos, cluster *v1alpha1.AtomixCluster) *Controller {
	logger := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace)
	return &Controller{logger, client, scheme, chaos, cluster}
}

type Controller struct {
	logger  logr.Logger
	client  client.Client
	scheme  *runtime.Scheme
	chaos   *chaos.Chaos
	cluster *v1alpha1.AtomixCluster
}

func (c *Controller) Reconcile() error {
	for _, config := range c.cluster.Spec.Chaos.Monkeys {
		monkey := c.chaos.GetMonkey(c.cluster, &config)
		if !monkey.Started {
			monkey.Start()
		}
	}

	for _, monkey := range c.chaos.GetMonkeys() {
		if monkey.Started {
			match := false
			for _, config := range c.cluster.Spec.Chaos.Monkeys {
				if monkey.Name == config.Name {
					match = true
					break
				}
			}
			if !match {
				monkey.Stop()
			}
		}
	}
	return nil
}
