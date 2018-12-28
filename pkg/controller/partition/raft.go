package partition

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RaftController struct {
	*Controller

	// Partition group spec
	group *v1alpha1.RaftPartitionGroup
}

func (c *RaftController) addInitScript() error {
	c.logger.Info("Creating new init script ConfigMap")
	cm := util.NewPartitionGroupInitConfigMap(c.cluster, c.Name)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *RaftController) addConfig() error {
	c.logger.Info("Creating new configuration ConfigMap")
	cm := util.NewRaftPartitionGroupConfigMap(c.cluster, c.Name, c.group)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *RaftController) addStatefulSet() error {
	c.logger.Info("Creating new StatefulSet")
	set := util.NewPersistentPartitionGroupStatefulSet(c.cluster, c.Name, &c.group.PersistentPartitionGroup)
	if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), set)
}
