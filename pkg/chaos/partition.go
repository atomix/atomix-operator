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
	"github.com/go-logr/logr"
)

type PartitionMonkey struct {
	MonkeyHandler
	config *v1alpha1.PartitionMonkey
	logger logr.Logger
}

type PartitionIsolateMonkey struct {
	*PartitionMonkey
}

func (m *PartitionIsolateMonkey) run(stop <-chan struct{}) {
	m.logger.Info("Partitioning (isolate) node")
	<-stop
}

type PartitionBridgeMonkey struct {
	*PartitionMonkey
}

func (m *PartitionBridgeMonkey) run(stop <-chan struct{}) {
	m.logger.Info("Partitioning (bridge) node")
	<-stop
}
