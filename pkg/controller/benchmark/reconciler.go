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

package benchmark

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

var log = logf.Log.WithName("controller_benchmark")

// AddController creates a new BenchmarkReconciler and adds it to the Manager. The Manager will set fields on the reconciler
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &BenchmarkReconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mgr.GetConfig(),
	}

	// Create a new controller
	c, err := controller.New("benchmark-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AtomixBenchmark
	err = c.Watch(&source.Kind{Type: &v1alpha1.AtomixBenchmark{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner AtomixBenchmark
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AtomixBenchmark{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &BenchmarkReconciler{}

// BenchmarkReconciler reconciles a AtomixCluster object
type BenchmarkReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	config *rest.Config
}

// Reconcile reads that state of the cluster for a AtomixBenchmark object and makes changes based on the state read
// and what is in the AtomixBenchmark.Spec
func (r *BenchmarkReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	logger.Info("Reconciling AtomixBenchmark")

	// Fetch the AtomixBenchmark instance
	benchmark := &v1alpha1.AtomixBenchmark{}
	err := r.client.Get(context.TODO(), request.NamespacedName, benchmark)
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

	v1alpha1.SetDefaults_Benchmark(benchmark)

	// Reconcile the init script
	err = r.reconcileControllerInitScript(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the system configuration
	err = r.reconcileControllerSystemConfig(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the StatefulSet
	err = r.reconcileControllerPod(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the cluster service
	err = r.reconcileControllerService(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the init script
	err = r.reconcileWorkerInitScript(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the system configuration
	err = r.reconcileWorkerSystemConfig(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the StatefulSet
	err = r.reconcileWorkerPod(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the cluster service
	err = r.reconcileWorkerService(benchmark)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *BenchmarkReconciler) getControllerInitScriptName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkControllerInitConfigMapName(benchmark)
}

func (r *BenchmarkReconciler) reconcileControllerInitScript(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerInitScriptName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addControllerInitScript(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addControllerInitScript(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerInitScriptName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new benchmark controller init script ConfigMap", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		cm = util.NewBenchmarkControllerInitConfigMap(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, cm, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), cm)
	}
	return err
}

func (r *BenchmarkReconciler) getControllerSystemConfigName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkControllerSystemConfigMapName(benchmark)
}

func (r *BenchmarkReconciler) reconcileControllerSystemConfig(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerSystemConfigName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addControllerSystemConfig(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addControllerSystemConfig(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerSystemConfigName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new benchmark controller system ConfigMap", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		cm = util.NewBenchmarkControllerSystemConfigMap(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, cm, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), cm)
	}
	return err
}

func (r *BenchmarkReconciler) getControllerPodName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkControllerPodName(benchmark)
}

func (r *BenchmarkReconciler) reconcileControllerPod(benchmark *v1alpha1.AtomixBenchmark) error {
	set := &corev1.Pod{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerPodName(benchmark), Namespace: benchmark.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = r.addControllerPod(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addControllerPod(benchmark *v1alpha1.AtomixBenchmark) error {
	pod := &corev1.Pod{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerPodName(benchmark), Namespace: benchmark.Namespace}, pod)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new benchmark controller pod", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		pod = util.NewBenchmarkControllerPod(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, pod, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), pod)
	}
	return err
}

func (r *BenchmarkReconciler) getControllerServiceName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkControllerServiceName(benchmark)
}

func (r *BenchmarkReconciler) reconcileControllerService(benchmark *v1alpha1.AtomixBenchmark) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerServiceName(benchmark), Namespace: benchmark.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = r.addControllerService(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addControllerService(benchmark *v1alpha1.AtomixBenchmark) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getControllerServiceName(benchmark), Namespace: benchmark.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new coordinator Service", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		service = util.NewBenchmarkControllerService(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, service, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), service)
	}
	return err
}

func (r *BenchmarkReconciler) getWorkerInitScriptName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkWorkerInitConfigMapName(benchmark)
}

func (r *BenchmarkReconciler) reconcileWorkerInitScript(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerInitScriptName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addWorkerInitScript(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addWorkerInitScript(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerInitScriptName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new worker init script ConfigMap", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		cm = util.NewBenchmarkWorkerInitConfigMap(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, cm, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), cm)
	}
	return err
}

func (r *BenchmarkReconciler) getWorkerSystemConfigName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkWorkerSystemConfigMapName(benchmark)
}

func (r *BenchmarkReconciler) reconcileWorkerSystemConfig(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerSystemConfigName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = r.addWorkerSystemConfig(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addWorkerSystemConfig(benchmark *v1alpha1.AtomixBenchmark) error {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerSystemConfigName(benchmark), Namespace: benchmark.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new worker system ConfigMap", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		cm = util.NewBenchmarkWorkerSystemConfigMap(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, cm, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), cm)
	}
	return err
}

func (r *BenchmarkReconciler) getWorkerStatefulSetName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkWorkerStatefulSetName(benchmark)
}

func (r *BenchmarkReconciler) reconcileWorkerPod(benchmark *v1alpha1.AtomixBenchmark) error {
	set := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerStatefulSetName(benchmark), Namespace: benchmark.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = r.addWorkerPod(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addWorkerPod(benchmark *v1alpha1.AtomixBenchmark) error {
	set := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerStatefulSetName(benchmark), Namespace: benchmark.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new worker StatefulSet", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		set = util.NewBenchmarkWorkerStatefulSet(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, set, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), set)
	}
	return err
}

func (r *BenchmarkReconciler) getWorkerServiceName(benchmark *v1alpha1.AtomixBenchmark) string {
	return util.GetBenchmarkWorkerServiceName(benchmark)
}

func (r *BenchmarkReconciler) reconcileWorkerService(benchmark *v1alpha1.AtomixBenchmark) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerServiceName(benchmark), Namespace: benchmark.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = r.addWorkerService(benchmark)
	}
	return err
}

func (r *BenchmarkReconciler) addWorkerService(benchmark *v1alpha1.AtomixBenchmark) error {
	service := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: r.getWorkerServiceName(benchmark), Namespace: benchmark.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating new benchmark worker Service", "Name", benchmark.Name, "Namespace", benchmark.Namespace)
		service = util.NewBenchmarkWorkerService(benchmark)
		if err := controllerutil.SetControllerReference(benchmark, service, r.scheme); err != nil {
			return err
		}
		return r.client.Create(context.TODO(), service)
	}
	return err
}
