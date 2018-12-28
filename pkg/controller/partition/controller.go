package partition

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller_atomix")

func New(client client.Client, scheme *runtime.Scheme, cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroupSpec) *Controller {
	lg := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace, "PartitionGroup", name)

	return &Controller{
		logger: lg,
		client: client,
		scheme: scheme,
		cluster: cluster,
		Name: name,
		group: *group,
	}
}

type Interface interface {
	getServiceName() string
	addService() error
	removeService() error
	getInitScriptName() string
	addInitScript() error
	removeInitScript() error
	getConfigName() string
	addConfig() error
	removeConfig() error
	getStatefulSetName() string
	addStatefulSet() error
	removeStatefulSet() error
	Reconcile() error
	Delete() error
}

type Controller struct {
	Interface
	logger logr.Logger
	client client.Client
	scheme *runtime.Scheme
	cluster *v1alpha1.AtomixCluster
    Name string
	group v1alpha1.PartitionGroupSpec
}

func (c *Controller) getServiceName() string {
	return util.GetPartitionGroupServiceName(c.cluster, c.Name)
}

func (c *Controller) addService() error {
	c.logger.Info("Creating new partition service")
	service := util.NewPartitionGroupService(c.cluster, c.Name)
	if err := controllerutil.SetControllerReference(c.cluster, service, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), service)
}

func (c *Controller) removeService() error {
	set := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getServiceName(), Namespace: c.cluster.Namespace}, set)
	if err == nil {
		c.logger.Info("Removing partition group service")
		return c.client.Delete(context.TODO(), set)
	} else if !errors.IsNotFound(err) {
		return err
	} else {
		c.logger.Info("Partition group service has already been removed")
	}
	return nil
}

func (c *Controller) getInitScriptName() string {
	return util.GetPartitionGroupInitConfigMapName(c.cluster, c.Name)
}

func (c *Controller) addInitScript() error {
	c.logger.Info("Creating new init script ConfigMap")
	cm := util.NewPartitionGroupInitConfigMap(c.cluster, c.Name)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) removeInitScript() error {
	set := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getInitScriptName(), Namespace: c.cluster.Namespace}, set)
	if err == nil {
		c.logger.Info("Removing init script ConfigMap")
		return c.client.Delete(context.TODO(), set)
	} else if !errors.IsNotFound(err) {
		return err
	} else {
		c.logger.Info("Init script ConfigMap has already been removed")
	}
	return nil
}

func (c *Controller) getConfigName() string {
	return util.GetPartitionGroupSystemConfigMapName(c.cluster, c.Name)
}

func (c *Controller) addConfig() error {
	c.logger.Info("Creating new configuration ConfigMap")
	cm, err := util.NewPartitionGroupConfigMap(c.cluster, &c.group)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *Controller) removeConfig() error {
	set := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getConfigName(), Namespace: c.cluster.Namespace}, set)
	if err == nil {
		c.logger.Info("Removing config ConfigMap")
		return c.client.Delete(context.TODO(), set)
	} else if !errors.IsNotFound(err) {
		return err
	} else {
		c.logger.Info("Config ConfigMap has already been removed")
	}
	return nil
}

func (c *Controller) getStatefulSetName() string {
	return util.GetPartitionGroupStatefulSetName(c.cluster, c.Name)
}

func (c *Controller) addStatefulSet() error {
	c.logger.Info("Creating new StatefulSet")
	set, err := util.NewPartitionGroupStatefulSet(c.cluster, &c.group)
	if err != nil {
		return err
	}
	if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), set)
}

func (c *Controller) removeStatefulSet() error {
	set := &appsv1.StatefulSet{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getStatefulSetName(), Namespace: c.cluster.Namespace}, set)
	if err == nil {
		c.logger.Info("Removing partition group StatefulSet")
		return c.client.Delete(context.TODO(), set)
	} else if !errors.IsNotFound(err) {
		return err
	} else {
		c.logger.Info("Partition group StatefulSet has already been removed")
	}
	return nil
}

// Delete deletes a group by name
func (c *Controller) Delete() error {
	err := c.removeService()
	if err != nil {
		return err
	}

	err = c.removeStatefulSet()
	if err != nil {
		return err
	}

	err = c.removeConfig()
	if err != nil {
		return err
	}

	err = c.removeInitScript()
	if err != nil {
		return err
	}
	return nil
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

	err = c.reconcileStatefulSet()
	if err != nil {
		return err
	}

	return c.reconcileService()
}

func (c *Controller) reconcileInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addInitScript()
	}
	return err
}

func (c *Controller) reconcileSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addConfig()
	}
	return err
}

func (c *Controller) reconcileStatefulSet() error {
	set := &appsv1.StatefulSet{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getStatefulSetName(), Namespace: c.cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = c.addStatefulSet()
	}
	return err
}

func (c *Controller) reconcileService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getServiceName(), Namespace: c.cluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		err = c.addService()
	}
	return err
}
