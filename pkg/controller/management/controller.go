package management

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller_atomix")

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
	err := c.reconcileInitScript()
	if err != nil {
		return err
	}

	err = c.reconcileSystemConfig()
	if err != nil {
		return err
	}

	err = c.reconcileDisruptionBudget()
	if err != nil {
		return err
	}

	err = c.reconcileStatefulSet()
	if err != nil {
		return err
	}

	err = c.reconcileService()
	if err != nil {
		return err
	}
	return err
}

func (c *Controller) reconcileInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementInitConfigMapName(c.cluster), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addInitScript()
	}
	return err
}

func (c *Controller) reconcileSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementSystemConfigMapName(c.cluster), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addConfig()
	}
	return err
}

func (c *Controller) reconcileDisruptionBudget() error {
	budget := &v1beta1.PodDisruptionBudget{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementDisruptionBudgetName(c.cluster), Namespace: c.cluster.Namespace}, budget)
	if err != nil && errors.IsNotFound(err) {
		err = c.addDisruptionBudget()
	}
	return err
}

func (c *Controller) reconcileStatefulSet() error {
	set := &appsv1.StatefulSet{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementStatefulSetName(c.cluster), Namespace: c.cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = c.addStatefulSet()
	}
	return err
}

func (c *Controller) reconcileService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: util.GetManagementServiceName(c.cluster), Namespace: c.cluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = c.addService()
	}
	return err
}

func (c *Controller) addInitScript() error {
	c.logger.Info("Creating new init script ConfigMap")
	cm := util.NewManagementInitConfigMap(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) addConfig() error {
	c.logger.Info("Creating new configuration ConfigMap")
	cm := util.NewManagementSystemConfigMap(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) addStatefulSet() error {
	c.logger.Info("Creating new management set")
	set, err := util.NewManagementStatefulSet(c.cluster)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), set)
}

func (c *Controller) addService() error {
	c.logger.Info("Creating new management service")
	service := util.NewManagementService(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, service, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), service)
}

func (c *Controller) addDisruptionBudget() error {
	c.logger.Info("Creating new pod disruption budget")
	budget := util.NewManagementDisruptionBudget(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, budget, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), budget)
}
