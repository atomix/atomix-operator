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

package partitiongroup

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_partitiongroup")

// AddController creates a new PartitionGroup controller and adds it to the Manager. The Manager will set fields on the
// controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &PartitionGroupReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("partitiongroup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PartitionGroup
	err = c.Watch(&source.Kind{Type: &v1alpha1.PartitionGroup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner PartitionGroup
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.PartitionGroup{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &PartitionGroupReconciler{}

// PartitionGroupReconciler reconciles a PartitionGroup object
type PartitionGroupReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a PartitionGroup object and makes changes based on the state read
// and what is in the PartitionGroup.Spec
func (r *PartitionGroupReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.Info("Reconciling PartitionGroup")

	// Fetch the PartitionGroup instance
	group := &v1alpha1.PartitionGroup{}
	err := r.client.Get(context.TODO(), request.NamespacedName, group)
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

	v1alpha1.SetDefaults_PartitionGroup(group)

	// Reconcile the init script
	err = r.reconcileInitScript(group)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the system configuration
	err = r.reconcileSystemConfig(group)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the pod disruption budget
	err = r.reconcileDisruptionBudget(group)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the StatefulSet
	err = r.reconcileStatefulSet(group)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the group service
	err = r.reconcileService(group)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *PartitionGroupReconciler) getInitScriptName(group *v1alpha1.PartitionGroup) string {
	return util.GetPartitionGroupInitConfigMapName(group)
}

func (r *PartitionGroupReconciler) reconcileInitScript(group *v1alpha1.PartitionGroup) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getInitScriptName(group), Namespace: group.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addInitScript(group)
	}
	return err
}

func (r *PartitionGroupReconciler) addInitScript(group *v1alpha1.PartitionGroup) error {
	log.Info("Creating new init script ConfigMap", "Name", group.Name, "Namespace", group.Namespace)
	cm := util.NewPartitionGroupInitConfigMap(group)
	if err := controllerutil.SetControllerReference(group, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

func (r *PartitionGroupReconciler) getSystemConfigName(group *v1alpha1.PartitionGroup) string {
	return util.GetPartitionGroupSystemConfigMapName(group)
}

func (r *PartitionGroupReconciler) reconcileSystemConfig(group *v1alpha1.PartitionGroup) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getSystemConfigName(group), Namespace: group.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addSystemConfig(group)
	}
	return err
}

func (r *PartitionGroupReconciler) addSystemConfig(group *v1alpha1.PartitionGroup) error {
	log.Info("Creating new configuration ConfigMap", "Name", group.Name, "Namespace", group.Namespace)
	cm, err := util.NewPartitionGroupConfigMap(group)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(group, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

func (r *PartitionGroupReconciler) getDisruptionBudgetName(group *v1alpha1.PartitionGroup) string {
	return util.GetPartitionGroupDisruptionBudgetName(group)
}

func (r *PartitionGroupReconciler) reconcileDisruptionBudget(group *v1alpha1.PartitionGroup) error {
	budget := &v1beta1.PodDisruptionBudget{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getDisruptionBudgetName(group), Namespace: group.Namespace}, budget)
	if err != nil && errors.IsNotFound(err) {
		err = r.addDisruptionBudget(group)
	}
	return err
}

func (r *PartitionGroupReconciler) addDisruptionBudget(group *v1alpha1.PartitionGroup) error {
	log.Info("Creating new pod disruption budget", "Name", group.Name, "Namespace", group.Namespace)
	budget := util.NewPartitionGroupDisruptionBudget(group)
	if err := controllerutil.SetControllerReference(group, budget, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), budget)
}

func (r *PartitionGroupReconciler) getStatefulSetName(group *v1alpha1.PartitionGroup) string {
	return util.GetPartitionGroupStatefulSetName(group)
}

func (r *PartitionGroupReconciler) reconcileStatefulSet(group *v1alpha1.PartitionGroup) error {
	set := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getStatefulSetName(group), Namespace: group.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = r.addStatefulSet(group)
	}
	return err
}

func (r *PartitionGroupReconciler) addStatefulSet(group *v1alpha1.PartitionGroup) error {
	log.Info("Creating new PartitionGroup set", "Name", group.Name, "Namespace", group.Namespace)
	set, err := util.NewPartitionGroupStatefulSet(group)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(group, set, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), set)
}

func (r *PartitionGroupReconciler) getServiceName(group *v1alpha1.PartitionGroup) string {
	return util.GetPartitionGroupServiceName(group)
}

func (r *PartitionGroupReconciler) reconcileService(group *v1alpha1.PartitionGroup) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getServiceName(group), Namespace: group.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = r.addService(group)
	}
	return err
}

func (r *PartitionGroupReconciler) addService(group *v1alpha1.PartitionGroup) error {
	log.Info("Creating new PartitionGroup service", "Name", group.Name, "Namespace", group.Namespace)
	service := util.NewPartitionGroupService(group)
	if err := controllerutil.SetControllerReference(group, service, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), service)
}
