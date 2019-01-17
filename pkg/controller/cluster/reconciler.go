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

package cluster

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

var log = logf.Log.WithName("controller_atomixcluster")

// AddController creates a new AtomixCluster ManagementGroup and adds it to the Manager. The Manager will set fields on the ManagementGroup
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &AtomixClusterReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
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

var _ reconcile.Reconciler = &AtomixClusterReconciler{}

// ReconcileAtomixCluster reconciles a AtomixCluster object
type AtomixClusterReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a AtomixCluster object and makes changes based on the state read
// and what is in the AtomixCluster.Spec
func (r *AtomixClusterReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.Info("Reconciling AtomixCluster")

	// Fetch the AtomixCluster instance
	cluster := &v1alpha1.AtomixCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, cluster)
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

	v1alpha1.SetDefaults_Cluster(cluster)

	// Reconcile the init script
	err = r.reconcileInitScript(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the system configuration
	err = r.reconcileSystemConfig(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the pod disruption budget
	err = r.reconcileDisruptionBudget(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the StatefulSet
	err = r.reconcileStatefulSet(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the cluster service
	err = r.reconcileService(cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *AtomixClusterReconciler) reconcileInitScript(cluster *v1alpha1.AtomixCluster) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementInitConfigMapName(cluster), Namespace: cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addInitScript(cluster)
	}
	return err
}

func (r *AtomixClusterReconciler) reconcileSystemConfig(cluster *v1alpha1.AtomixCluster) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementSystemConfigMapName(cluster), Namespace: cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addConfig(cluster)
	}
	return err
}

func (r *AtomixClusterReconciler) reconcileDisruptionBudget(cluster *v1alpha1.AtomixCluster) error {
	budget := &v1beta1.PodDisruptionBudget{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementDisruptionBudgetName(cluster), Namespace: cluster.Namespace}, budget)
	if err != nil && errors.IsNotFound(err) {
		err = r.addDisruptionBudget(cluster)
	}
	return err
}

func (r *AtomixClusterReconciler) reconcileStatefulSet(cluster *v1alpha1.AtomixCluster) error {
	set := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementStatefulSetName(cluster), Namespace: cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = r.addStatefulSet(cluster)
	}
	return err
}

func (r *AtomixClusterReconciler) reconcileService(cluster *v1alpha1.AtomixCluster) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementServiceName(cluster), Namespace: cluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = r.addService(cluster)
	}
	return err
}

func (r *AtomixClusterReconciler) addInitScript(cluster *v1alpha1.AtomixCluster) error {
	log.Info("Creating new init script ConfigMap", "Name", cluster.Name, "Namespace", cluster.Namespace)
	cm := util.NewManagementInitConfigMap(cluster)
	if err := controllerutil.SetControllerReference(cluster, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

func (r *AtomixClusterReconciler) addConfig(cluster *v1alpha1.AtomixCluster) error {
	log.Info("Creating new configuration ConfigMap", "Name", cluster.Name, "Namespace", cluster.Namespace)
	cm := util.NewManagementSystemConfigMap(cluster)
	if err := controllerutil.SetControllerReference(cluster, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

func (r *AtomixClusterReconciler) addStatefulSet(cluster *v1alpha1.AtomixCluster) error {
	log.Info("Creating new management set", "Name", cluster.Name, "Namespace", cluster.Namespace)
	set, err := util.NewManagementStatefulSet(cluster)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(cluster, set, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), set)
}

func (r *AtomixClusterReconciler) addService(cluster *v1alpha1.AtomixCluster) error {
	log.Info("Creating new management service", "Name", cluster.Name, "Namespace", cluster.Namespace)
	service := util.NewManagementService(cluster)
	if err := controllerutil.SetControllerReference(cluster, service, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), service)
}

func (r *AtomixClusterReconciler) addDisruptionBudget(cluster *v1alpha1.AtomixCluster) error {
	log.Info("Creating new pod disruption budget", "Name", cluster.Name, "Namespace", cluster.Namespace)
	budget := util.NewManagementDisruptionBudget(cluster)
	if err := controllerutil.SetControllerReference(cluster, budget, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), budget)
}
