package partition

import (
	"context"
	"github.com/atomix/atomix-operator/pkg/apis/agent/v1alpha1"
	"github.com/atomix/atomix-operator/pkg/controller/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PrimaryBackupController struct {
	*Controller

	// Partition group spec
	group *v1alpha1.PrimaryBackupPartitionGroup
}

func (c *PrimaryBackupController) addInitScript() error {
	c.logger.Info("Creating new init script ConfigMap")
	cm := util.NewInitConfigMap(c.cluster)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *PrimaryBackupController) addConfig() error {
	c.logger.Info("Creating new configuration ConfigMap")
	cm := util.NewPrimaryBackupPartitionGroupConfigMap(c.cluster, c.Name, c.group)
	if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), cm)
}

func (c *PrimaryBackupController) addStatefulSet() error {
	c.logger.Info("Creating new StatefulSet")
	set := util.NewEphemeralPartitionGroupStatefulSet(c.cluster, c.Name, &c.group.PartitionGroup)
	if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
		return err
	}
	return c.client.Create(context.TODO(), set)
}
