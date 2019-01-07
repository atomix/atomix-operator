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

package controller

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/chaos"
	"github.com/atomix/atomix-operator/pkg/controller/benchmark"
	chaoscontroller "github.com/atomix/atomix-operator/pkg/controller/chaos"
	"github.com/atomix/atomix-operator/pkg/controller/management"
	"github.com/atomix/atomix-operator/pkg/controller/partition"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_atomix")

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, AddController)
}

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}

// AddController creates a new AtomixCluster ManagementGroup and adds it to the Manager. The Manager will set fields on the ManagementGroup
// and Start it when the Manager is Started.
func AddController(mgr manager.Manager) error {
	chaos := chaos.New(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig())
	mgr.Add(chaos)

	r := &ReconcileAtomixCluster{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
		chaos:  chaos,
	}

	// Create a new controller
	c, err := controller.New("atomixcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AtomixCluster
	err = c.Watch(&source.Kind{Type: &v1alpha1.AtomixCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner AtomixCluster
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AtomixCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileAtomixCluster{}

// ReconcileAtomixCluster reconciles a AtomixCluster object
type ReconcileAtomixCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
	chaos  *chaos.Chaos
}

// Reconcile reads that state of the cluster for a AtomixCluster object and makes changes based on the state read
// and what is in the AtomixCluster.Spec
func (r *ReconcileAtomixCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling AtomixCluster")

	// Fetch the AtomixCluster instance
	instance := &v1alpha1.AtomixCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	v1alpha1.SetDefaults_Cluster(instance)
	err = New(r.client, r.scheme, r.config, r.chaos, instance).Reconcile()
	return reconcile.Result{}, err
}

func New(client client.Client, scheme *runtime.Scheme, config *rest.Config, chaos *chaos.Chaos, cluster *v1alpha1.AtomixCluster) *Controller {
	logger := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace)
	return &Controller{logger, client, scheme, config, chaos, cluster}
}

type Controller struct {
	logger  logr.Logger
	client  client.Client
	scheme  *runtime.Scheme
	config  *rest.Config
	chaos   *chaos.Chaos
	cluster *v1alpha1.AtomixCluster
}

func (c *Controller) Reconcile() error {
	err := c.reconcileManagementGroup()
	if err != nil {
		return err
	}

	err = c.reconcilePartitionGroups()
	if err != nil {
		return err
	}

	err = c.reconcileBenchmark()
	if err != nil {
		return err
	}

	err = c.reconcileChaosMonkeys()
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) reconcileManagementGroup() error {
	return management.New(c.client, c.scheme, c.cluster).Reconcile()
}

func (c *Controller) reconcilePartitionGroups() error {
	for _, group := range c.cluster.Spec.PartitionGroups {
		err := partition.New(c.client, c.scheme, c.cluster, group.Name, &group).Reconcile()
		if err != nil {
			return err
		}
	}

	selector := labels.SelectorFromSet(util.NewPartitionGroupClusterLabels(c.cluster))

	listOptions := client.ListOptions{
		Namespace:     c.cluster.Namespace,
		LabelSelector: selector,
	}

	setList := &appsv1.StatefulSetList{}
	err := c.client.List(context.TODO(), &listOptions, setList)
	if err != nil {
		return err
	}

	for _, set := range setList.Items {
		if groupName, ok := set.Labels["group"]; ok {
			found := v1alpha1.PartitionGroupSpec{}
			for _, group := range c.cluster.Spec.PartitionGroups {
				if group.Name == groupName {
					found = group
					break
				}
			}
			if found.Name == "" {
				err = partition.New(c.client, c.scheme, c.cluster, groupName, &v1alpha1.PartitionGroupSpec{}).Delete()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Controller) reconcileBenchmark() error {
	return benchmark.New(c.client, c.scheme, c.cluster).Reconcile()
}

func (c *Controller) reconcileChaosMonkeys() error {
	return chaoscontroller.New(c.client, c.scheme, c.chaos, c.cluster).Reconcile()
}
