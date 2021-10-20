/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	syntheticsv1 "github.com/kentik/odyssey/api/v1"
)

const (
	defaultSyntheticServerImage      = "docker.io/kentik/synsrv:latest"
	ownerKey                         = ".metadata.controller"
	controllerLabel                  = "kentiklabs.controllers.ksynth"
	serverDeploymentConfigVolumeName = "config"
	serverDeploymentContainerName    = "synthetic-server"
	serverConfigMapName              = "server-config.yml"
)

var (
	baseServerConfig = `
tasks:
  - ping:
      target: 127.0.0.1
      count: 1
      delay: 0s
      period: 30s
      expiry: 2s
    ipv4: true
    ipv6: false
`
)

// SyntheticTaskReconciler reconciles a SyntheticTask object
type SyntheticTaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SyntheticTask object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SyntheticTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace)

	// your logic here
	var task syntheticsv1.SyntheticTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.V(1).Info("checking deployment")
	// ksynth server deployment
	currentDeployment := appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, &currentDeployment); err != nil {
		// no deployment found; create
		if apierrors.IsNotFound(err) {
			log.V(1).Info("creating server deployment")
			defaultConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverConfigMapName,
					Namespace: req.NamespacedName.Namespace,
				},
				Data: map[string]string{
					serverConfigMapName: baseServerConfig,
				},
			}
			ctrl.SetControllerReference(&task, defaultConfigMap, r.Scheme)
			if err := r.Create(ctx, defaultConfigMap); err != nil {
				return ctrl.Result{}, err
			}
			deployment := r.getServerDeployment(&task, defaultConfigMap)
			ctrl.SetControllerReference(&task, deployment, r.Scheme)
			if err := r.Create(ctx, deployment); err != nil {
				return ctrl.Result{}, err
			}
			// deployed; requeue and wait
			return ctrl.Result{Requeue: true}, nil
		}

		log.Error(err, "error getting server deployment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// build and update config for server
	log.Info("checking tasks")
	for _, fetch := range task.Spec.Fetch {
		log.V(0).Info("adding fetch", "fetch", fmt.Sprintf("%+v", fetch))
		var svc corev1.Service
		if err := r.Get(ctx, types.NamespacedName{Name: fetch.Service, Namespace: req.NamespacedName.Namespace}, &svc); err != nil {
			return ctrl.Result{}, err
		}
		// TODO: build target with service IP
		log.Info("building fetch configuration for service", "service", svc.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyntheticTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &syntheticsv1.SyntheticTask{}, ownerKey, func(rawObj client.Object) []string {
		task := rawObj.(*syntheticsv1.SyntheticTask)
		owner := metav1.GetControllerOf(task)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != syntheticsv1.GroupVersion.String() || owner.Kind != "SyntheticTask" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&syntheticsv1.SyntheticTask{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *SyntheticTaskReconciler) getServerDeployment(t *syntheticsv1.SyntheticTask, configMap *corev1.ConfigMap) *appsv1.Deployment {
	replicas := int32(1)
	labels := appLabels(t.Name)
	image := t.Spec.Image
	if image == "" {
		image = defaultSyntheticServerImage
	}
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Image: image,
				Name:  serverDeploymentContainerName,
				Command: []string{
					"synsrv",
					"-v",
					"server",
					"-b",
					"0.0.0.0:8080",
					"-c",
					fmt.Sprintf("/etc/kentik/%s", serverConfigMapName),
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "server",
						ContainerPort: 8080,
					},
				},
			},
		},
	}
	if configMap != nil {
		podSpec.Volumes = []corev1.Volume{
			{
				Name: serverDeploymentConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{

							Name: configMap.Name,
						},
					},
				},
			},
		}
		podSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      serverDeploymentConfigVolumeName,
				ReadOnly:  true,
				MountPath: "/etc/kentik",
			},
		}
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}

	return deploy
}

func appLabels(name string) map[string]string {
	return map[string]string{"controller": controllerLabel, "name": name}
}
