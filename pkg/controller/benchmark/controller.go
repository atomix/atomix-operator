package benchmark

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

func New(client client.Client, scheme *runtime.Scheme, cluster *v1alpha1.AtomixCluster) *Controller {
	lg := log.WithValues("Cluster", cluster.Name, "Namespace", cluster.Namespace)
	return &Controller{lg, client, scheme, cluster}
}

type Controller struct {
	logger  logr.Logger
	client  client.Client
	scheme  *runtime.Scheme
	cluster *v1alpha1.AtomixCluster
}

func (c *Controller) Reconcile() error {
	if c.cluster.Spec.Benchmark != nil {
		set := &appsv1.StatefulSet{}
		err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerStatefulSetName(), Namespace: c.cluster.Namespace}, set)
		if err != nil {
			if errors.IsNotFound(err) {
				return c.create()
			}
			return err
		} else if *set.Spec.Replicas != c.cluster.Spec.Benchmark.Size {
			err = c.delete()
			if err != nil {
				return err
			}
		}
		return c.create()
	} else {
		return c.delete()
	}
}

func (c *Controller) create() error {
	err := c.addWorkerInitScript()
	if err != nil {
		return err
	}

	err = c.addWorkerSystemConfig()
	if err != nil {
		return err
	}

	err = c.addWorkerStatefulSet()
	if err != nil {
		return err
	}

	err = c.addWorkerService()
	if err != nil {
		return err
	}

	err = c.addCoordinatorInitScript()
	if err != nil {
		return err
	}

	err = c.addCoordinatorSystemConfig()
	if err != nil {
		return err
	}

	err = c.addCoordinatorPod()
	if err != nil {
		return err
	}

	err = c.addCoordinatorService()
	if err != nil {
		return err
	}

	return err
}

func (c *Controller) delete() error {
	err := c.removeCoordinatorPod()
	if err != nil {
		return err
	}

	err = c.removeCoordinatorService()
	if err != nil {
		return err
	}

	err = c.removeCoordinatorSystemConfig()
	if err != nil {
		return err
	}

	err = c.removeCoordinatorInitScript()
	if err != nil {
		return err
	}

	err = c.removeWorkerStatefulSet()
	if err != nil {
		return err
	}

	err = c.removeWorkerService()
	if err != nil {
		return err
	}

	err = c.removeWorkerSystemConfig()
	if err != nil {
		return err
	}

	err = c.removeWorkerInitScript()
	if err != nil {
		return err
	}

	return err
}

func (c *Controller) getCoordinatorInitScriptName() string {
	return util.GetBenchmarkCoordinatorInitConfigMapName(c.cluster)
}

func (c *Controller) addCoordinatorInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new init script ConfigMap")
		cm = util.NewBenchmarkCoordinatorInitConfigMap(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), cm)
	}
	return err
}

func (c *Controller) removeCoordinatorInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err == nil {
		c.logger.Info("Deleting init script ConfigMap")
		return c.client.Delete(context.TODO(), cm)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getCoordinatorSystemConfigName() string {
	return util.GetBenchmarkCoordinatorSystemConfigMapName(c.cluster)
}

func (c *Controller) addCoordinatorSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorSystemConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new system ConfigMap")
		cm = util.NewBenchmarkCoordinatorSystemConfigMap(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), cm)
	}
	return err
}

func (c *Controller) removeCoordinatorSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorSystemConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err == nil {
		c.logger.Info("Deleting system ConfigMap")
		return c.client.Delete(context.TODO(), cm)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getCoordinatorPodName() string {
	return util.GetBenchmarkCoordinatorPodName(c.cluster)
}

func (c *Controller) addCoordinatorPod() error {
	pod := &corev1.Pod{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorPodName(), Namespace: c.cluster.Namespace}, pod)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new coordinator Pod")
		pod = util.NewBenchmarkCoordinatorPod(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, pod, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), pod)
	}
	return err
}

func (c *Controller) removeCoordinatorPod() error {
	pod := &corev1.Pod{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorPodName(), Namespace: c.cluster.Namespace}, pod)
	if err == nil {
		c.logger.Info("Deleting coordinator Pod")
		return c.client.Delete(context.TODO(), pod)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getCoordinatorServiceName() string {
	return util.GetBenchmarkCoordinatorServiceName(c.cluster)
}

func (c *Controller) addCoordinatorService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorServiceName(), Namespace: c.cluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new coordinator Service")
		service = util.NewBenchmarkCoordinatorService(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, service, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), service)
	}
	return err
}

func (c *Controller) removeCoordinatorService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getCoordinatorServiceName(), Namespace: c.cluster.Namespace}, service)
	if err == nil {
		c.logger.Info("Deleting coordinator Service")
		return c.client.Delete(context.TODO(), service)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getWorkerInitScriptName() string {
	return util.GetBenchmarkWorkerInitConfigMapName(c.cluster)
}

func (c *Controller) addWorkerInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new worker init script ConfigMap")
		cm = util.NewBenchmarkWorkerInitConfigMap(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), cm)
	}
	return err
}

func (c *Controller) removeWorkerInitScript() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerInitScriptName(), Namespace: c.cluster.Namespace}, cm)
	if err == nil {
		c.logger.Info("Deleting worker init script ConfigMap")
		return c.client.Delete(context.TODO(), cm)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getWorkerSystemConfigName() string {
	return util.GetBenchmarkWorkerSystemConfigMapName(c.cluster)
}

func (c *Controller) addWorkerSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerSystemConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new worker system ConfigMap")
		cm = util.NewBenchmarkWorkerSystemConfigMap(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, cm, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), cm)
	}
	return err
}

func (c *Controller) removeWorkerSystemConfig() error {
	cm := &corev1.ConfigMap{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerSystemConfigName(), Namespace: c.cluster.Namespace}, cm)
	if err == nil {
		c.logger.Info("Deleting worker system ConfigMap")
		return c.client.Delete(context.TODO(), cm)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getWorkerStatefulSetName() string {
	return util.GetBenchmarkWorkerStatefulSetName(c.cluster)
}

func (c *Controller) addWorkerStatefulSet() error {
	set := &appsv1.StatefulSet{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerStatefulSetName(), Namespace: c.cluster.Namespace}, set)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new worker StatefulSet")
		set = util.NewBenchmarkWorkerStatefulSet(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, set, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), set)
	}
	return err
}

func (c *Controller) removeWorkerStatefulSet() error {
	set := &appsv1.StatefulSet{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerStatefulSetName(), Namespace: c.cluster.Namespace}, set)
	if err == nil {
		c.logger.Info("Deleting worker StatefulSet")
		return c.client.Delete(context.TODO(), set)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Controller) getWorkerServiceName() string {
	return util.GetBenchmarkWorkerServiceName(c.cluster)
}

func (c *Controller) addWorkerService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerServiceName(), Namespace: c.cluster.Namespace}, service)
	if err != nil && errors.IsNotFound(err) {
		c.logger.Info("Creating new coordinator Service")
		service = util.NewBenchmarkWorkerService(c.cluster)
		if err := controllerutil.SetControllerReference(c.cluster, service, c.scheme); err != nil {
			return err
		}
		return c.client.Create(context.TODO(), service)
	}
	return err
}

func (c *Controller) removeWorkerService() error {
	service := &corev1.Service{}
	err := c.client.Get(context.TODO(), types.NamespacedName{Name: c.getWorkerServiceName(), Namespace: c.cluster.Namespace}, service)
	if err == nil {
		c.logger.Info("Deleting worker Service")
		return c.client.Delete(context.TODO(), service)
	} else if !errors.IsNotFound(err) {
		return err
	}
	return nil
}
