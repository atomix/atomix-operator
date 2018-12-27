package partition

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller_atomix")

func New(client client.Client, scheme *runtime.Scheme, cluster *v1alpha1.AtomixCluster, name string, group *v1alpha1.PartitionGroupSpec) Interface {
	groupType, err := v1alpha1.GetPartitionGroupType(group)
	if err != nil {
		return nil
	}

	lg := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace, "PartitionGroup", name)

	controller := &Controller{
		logger: lg,
		client: client,
		scheme: scheme,
		cluster: cluster,
		Name: name,
	}

	switch {
	case groupType == v1alpha1.RaftType:
		return &RaftController{controller, group.Raft}
	case groupType == v1alpha1.PrimaryBackupType:
		return &PrimaryBackupController{controller, group.PrimaryBackup}
	case groupType == v1alpha1.LogType:
		return &LogController{controller, group.Log}
	}
	return nil
}

type Interface interface {
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
}

type Controller struct {
	Interface
	logger logr.Logger
	client client.Client
	scheme *runtime.Scheme
	cluster *v1alpha1.AtomixCluster
    Name string
}

func (c *Controller) getInitScriptName() string {
	return c.cluster.Name + "-" + c.Name + "-init"
}

func (c *Controller) getConfigName() string {
	return c.cluster.Name + "-" + c.Name + "-config"
}

func (c *Controller) getStatefulSetName() string {
	return c.cluster.Name + "-" + c.Name
}

func (c *Controller) Reconcile() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addInitScript()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	cm = &corev1.ConfigMap{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Name: c.getConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		err = c.addConfig()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	set := &appsv1.StatefulSet{}
	err = c.client.Get(context.TODO(), types.NamespacedName{Name: c.getStatefulSetName(), Namespace: c.cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		err = c.addStatefulSet()
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
