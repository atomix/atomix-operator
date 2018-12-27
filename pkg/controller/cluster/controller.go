package cluster

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/partition"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_atomix")

// Add creates a new AtomixCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAtomixCluster{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

	err = New(r.client, r.scheme, instance).Reconcile()
	return reconcile.Result{}, err
}

func New(client client.Client, scheme *runtime.Scheme, cluster *v1alpha1.AtomixCluster) *Controller {
	lg := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace)
	return &Controller{lg, client, scheme, cluster}
}

type Controller struct {
	logger logr.Logger
	client client.Client
	scheme *runtime.Scheme
	cluster *v1alpha1.AtomixCluster
}

func (c *Controller) Reconcile() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.cluster.Name + "-init", Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addInitScript()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	cm = &corev1.ConfigMap{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Name: c.cluster.Name + "-config", Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addConfig()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	set := &appsv1.StatefulSet{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Name: c.cluster.Name, Namespace: c.cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		c.addStatefulSet()
	} else if err != nil {
		return err
	}

	for name, group := range c.cluster.Spec.PartitionGroups {
		err = partition.New(c.client, c.scheme, c.cluster, name, &group).Reconcile()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) addInitScript() error {
	c.logger.Info("Creating new init script ConfigMap")
	cm := util.NewInitConfigMap(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) removeInitScript() error {
	// TODO
	return nil
}

func (c *Controller) addConfig() error {
	c.logger.Info("Creating new configuration ConfigMap")
	cm := util.NewControllerConfigMap(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) addStatefulSet() error {
	c.logger.Info("Creating new controller set")
	set := util.NewControllerStatefulSet(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), set)
}
