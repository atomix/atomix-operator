package controller

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/management"
	"github.com/atomix/atomix-operator/pkg/controller/partition"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_atomix")

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

// Add creates a new AtomixCluster ManagementGroup and adds it to the Manager. The Manager will set fields on the ManagementGroup
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAtomixCluster{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new ManagementGroup to mgr with r as the reconcile.Reconciler
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

	v1alpha1.SetDefaults_Cluster(instance)
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
	err := c.reconcileManagementGroup()
	if err != nil {
		return err
	}
	return c.reconcilePartitionGroups()
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

	setList := &appsv1.StatefulSetList{}
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement("app", selection.In, []string{c.cluster.Name})
	if err != nil {
		return err
	}

	selector.Add(*requirement)
	listOptions := client.ListOptions{
		Namespace: c.cluster.Namespace,
		LabelSelector: selector,
	}

	err = c.client.List(context.TODO(), &listOptions, setList)
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
