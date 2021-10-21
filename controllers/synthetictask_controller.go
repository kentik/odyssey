/*
Copyright 2021 KentikLabs

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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	defaultSyntheticServerImage      = "docker.io/kentiklabs/synsrv:latest"
	defaultSyntheticAgentImage       = "docker.io/kentik/ksynth:latest"
	ownerKey                         = ".metadata.controller"
	serverName                       = "synthetics-server"
	agentName                        = "synthetics-agent"
	serverLabel                      = "kentiklabs.synthetics.server"
	agentLabel                       = "kentiklabs.synthetics.agent"
	serverDeploymentConfigVolumeName = "config"
	serverDeploymentContainerName    = "synthetics-server"
	serverPortName                   = "server"
	serverPort                       = int32(8080)
	serverConfigMapName              = "server-config.yml"
	agentDeploymentContainerName     = "synthetics-agent"
	agentApiHostEnvVar               = "KENTIK_API_HOST"
)

var (
	defaultAgentCommand = []string{"ksynth", "agent", "-vv"}
	baseServerConfig    = `
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

type updateTask interface {
	// Name returns the name of the task
	Name() string
	// Yaml returns the task as a yaml config for the server
	Yaml() (string, error)
}
type updateConfig struct {
	syntheticTask    *syntheticsv1.SyntheticTask
	serverService    *corev1.Service
	serverConfigMap  *corev1.ConfigMap
	serverDeployment *appsv1.Deployment
	tasks            []updateTask
}

// SyntheticTaskReconciler reconciles a SyntheticTask object
type SyntheticTaskReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Log      logr.Logger
	tasks    map[string]interface{}
	updateCh chan *updateConfig
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
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SyntheticTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace)

	var task syntheticsv1.SyntheticTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// server config map
	currentServerConfigMap := corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: getServerConfigMapName(&task), Namespace: task.Namespace}, &currentServerConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("creating server configmap")
		configMap := r.getServerConfigMap(&task, baseServerConfig)
		ctrl.SetControllerReference(&task, configMap, r.Scheme)
		if err := r.Create(ctx, configMap); err != nil {
			log.Error(err, "error getting server configmap")
			return ctrl.Result{}, err
		}
		// created; requeue and wait
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("checking server deployment", "name", task.Name, "namespace", task.Namespace)
	// server deployment
	currentServerDeployment := appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: getServerDeploymentName(&task), Namespace: task.Namespace}, &currentServerDeployment); err != nil {
		// no deployment found; create
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting server deployment")
			return ctrl.Result{}, err
		}

		deployment := r.getServerDeployment(&task, &currentServerConfigMap)
		ctrl.SetControllerReference(&task, deployment, r.Scheme)
		if err := r.Create(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
		// deployed; requeue and wait
		return ctrl.Result{Requeue: true}, nil
	}

	// server service
	currentServerService := corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: getServerServiceName(&task), Namespace: task.Namespace}, &currentServerService); err != nil {
		// no service found; create
		if apierrors.IsNotFound(err) {
			log.Info("creating server service")
			deployment := r.getServerService(&task)
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

	log.Info("checking agent deployment", "name", task.Name, "namespace", task.Namespace)
	// ksynth agent deployment
	currentAgentDeployment := appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: getAgentDeploymentName(&task), Namespace: task.Namespace}, &currentAgentDeployment); err != nil {
		// no deployment found; create
		if apierrors.IsNotFound(err) {
			log.Info("creating agent deployment")
			deployment := r.getAgentDeployment(&task, &currentServerService)
			ctrl.SetControllerReference(&task, deployment, r.Scheme)
			if err := r.Create(ctx, deployment); err != nil {
				return ctrl.Result{}, err
			}
			// deployed; requeue and wait
			return ctrl.Result{Requeue: true}, nil
		}

		log.Error(err, "error getting agent deployment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// build and update config for server
	log.Info("checking tasks")
	updateTasks := []updateTask{}
	// fetch
	for _, fetch := range task.Spec.Fetch {
		log.V(0).Info("adding fetch", "fetch", fmt.Sprintf("%+v", fetch))
		var svc corev1.Service
		if err := r.Get(ctx, types.NamespacedName{Name: fetch.Service, Namespace: req.NamespacedName.Namespace}, &svc); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("building fetch configuration for service", "service", svc.Name)
		scheme := "http"
		if fetch.TLS {
			scheme = "https"
		}
		serviceEndpoint := fmt.Sprintf("%s://%s:%d%s", scheme, svc.Spec.ClusterIP, fetch.Port, fetch.Target)

		// override target with service info
		fetch.Target = serviceEndpoint

		// push the task to the controller list
		updateTasks = append(updateTasks, &fetch)
	}
	// tls handshake
	for _, tlsHandshake := range task.Spec.TLSHandshake {
		log.V(0).Info("adding tls handshake", "shake", fmt.Sprintf("%+v", tlsHandshake))
		var ingress networkingv1.Ingress
		if err := r.Get(ctx, types.NamespacedName{Name: tlsHandshake.Ingress, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("building tls handshake configuration for ingress", "ingress", ingress.Name)
		for _, rule := range ingress.Spec.Rules {
			log.Info("adding tls handshake rule", "host", rule.Host)
			tlsHandshake.Target = rule.Host
			updateTasks = append(updateTasks, &tlsHandshake)
		}
	}

	updateID := generateUpdateID(updateTasks)
	log.Info("checking update", "updateID", updateID, "current", task.Status.UpdateID)

	if task.Status.UpdateID != updateID {
		log.Info("update required", "task", task.Name)
		task.Status.UpdateID = updateID
		task.Status.DeployNeeded = true

		configData := "tasks:"
		for _, t := range updateTasks {
			yml, err := t.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		}

		currentServerConfigMap.Data[serverConfigMapName] = configData
		if err := r.Update(ctx, &currentServerConfigMap); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Status().Update(ctx, &task); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if task.Status.DeployNeeded {
		log.Info("deploy needed", "task", task.Name)
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(serverLabels(task.Name))); err != nil {
			return ctrl.Result{}, err
		}

		for _, pod := range podList.Items {
			log.Info("deleting pod", "pod", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
		task.Status.DeployNeeded = false
		if err := r.Status().Update(ctx, &task); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: cleanup old config maps

	log.Info("update and deployment complete", "task", task.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyntheticTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.tasks = map[string]interface{}{}
	r.updateCh = make(chan *updateConfig)

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

func generateUpdateID(tasks []updateTask) string {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Name() > tasks[j].Name()
	})

	h := sha1.New()
	for _, task := range tasks {
		h.Write([]byte(task.Name()))
	}

	return hex.EncodeToString(h.Sum(nil))
}
