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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	agentKentikCompanyEnvVar         = "KENTIK_COMPANY"
	agentKentikSiteEnvVar            = "KENTIK_SITE"
	agentKentikRegionEnvVar          = "KENTIK_REGION"
	agentKentikAgentUpdateEnvVar     = "AGENT_UPDATE"
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
	// ID returns the id of the task
	ID() string
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
	configData := "tasks:"
	// fetch
	for _, fetch := range task.Spec.Fetch {
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

		log.V(0).Info("adding fetch", "fetch", fmt.Sprintf("%+v", fetch))
		yml, err := fetch.Yaml()
		if err != nil {
			return ctrl.Result{}, err
		}
		configData += yml
	}
	// tls handshake
	for _, tlsHandshake := range task.Spec.TLSHandshake {
		var ingress networkingv1.Ingress
		if err := r.Get(ctx, types.NamespacedName{Name: tlsHandshake.Ingress, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("building tls handshake configuration for ingress", "ingress", ingress.Name)
		log.V(0).Info("adding tls handshake", "shake", fmt.Sprintf("%+v", tlsHandshake))
		for _, rule := range ingress.Spec.Rules {
			log.Info("adding tls handshake rule", "host", rule.Host)
			tlsHandshake.Target = rule.Host
			yml, err := tlsHandshake.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		}
	}
	// ping
	for _, ping := range task.Spec.Ping {
		log.Info("adding ping", "kind", ping.Kind, "name", ping.Name)
		switch strings.ToLower(ping.Kind) {
		case "deployment":
			log.Info("building ping configuration for deployment", "ping", ping.Name)
			var deploy appsv1.Deployment
			if err := r.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return ctrl.Result{}, err
			}
			podList := &corev1.PodList{}
			if err := r.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding ping", "deployment", deploy.Name)
			existingPods := map[string]struct{}{}
			for _, pod := range podList.Items {
				log.Info("adding ping", "pod", pod.Name)
				if pod.Status.PodIP == "" {
					// wait on pod ip
					log.Info("waiting on pod ip", "ping", ping.Name, "pod", pod.Name)
					return ctrl.Result{Requeue: true}, nil
				}
				if _, exists := existingPods[pod.Status.PodIP]; exists {
					log.Info("skipping existing pod", "pod", pod.Name)
					continue
				}
				ping.Target = pod.Status.PodIP
				yml, err := ping.Yaml()
				if err != nil {
					return ctrl.Result{}, err
				}
				configData += yml
			}
		case "pod":
			log.Info("building ping configuration for pod", "ping", ping.Name)
			var pod corev1.Pod
			if err := r.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
				return ctrl.Result{}, err
			}
			if pod.Status.PodIP == "" {
				// wait on pod ip
				log.Info("waiting on pod ip", "ping", ping.Name, "pod", pod.Name)
				return ctrl.Result{Requeue: true}, nil
			}
			log.Info("adding ping", "pod", pod.Name)
			ping.Target = pod.Status.PodIP
			yml, err := ping.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		case "service":
			log.Info("building ping configuration for service", "ping", ping.Name)
			var service corev1.Service
			if err := r.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding ping", "service", service.Name)
			ping.Target = service.Spec.ClusterIP
			yml, err := ping.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		case "ingress":
			log.Info("building ping configuration for ingress", "ping", ping.Name)
			var ingress networkingv1.Ingress
			if err := r.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding ping", "ingress", ingress.Name)
			for _, rule := range ingress.Spec.Rules {
				log.Info("adding ping rule", "host", rule.Host)
				ping.Target = rule.Host
				yml, err := ping.Yaml()
				if err != nil {
					return ctrl.Result{}, err
				}
				configData += yml
			}
		default:
			return ctrl.Result{}, fmt.Errorf("invalid ping kind %s", ping.Kind)
		}
	}

	// trace
	for _, trace := range task.Spec.Trace {
		log.Info("adding trace", "kind", trace.Kind, "name", trace.Name)
		switch strings.ToLower(trace.Kind) {
		case "deployment":
			log.Info("building trace configuration for deployment", "trace", trace.Name)
			var deploy appsv1.Deployment
			if err := r.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return ctrl.Result{}, err
			}
			podList := &corev1.PodList{}
			if err := r.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding trace", "deployment", deploy.Name)
			existingPods := map[string]struct{}{}
			for _, pod := range podList.Items {
				log.Info("adding trace", "pod", pod.Name)
				if pod.Status.PodIP == "" {
					// wait on pod ip
					log.Info("waiting on pod ip", "trace", trace.Name, "pod", pod.Name)
					return ctrl.Result{Requeue: true}, nil
				}
				if _, exists := existingPods[pod.Status.PodIP]; exists {
					log.Info("skipping existing pod", "pod", pod.Name)
					continue
				}
				trace.Target = pod.Status.PodIP
				yml, err := trace.Yaml()
				if err != nil {
					return ctrl.Result{}, err
				}
				configData += yml
			}
		case "pod":
			log.Info("building trace configuration for pod", "trace", trace.Name)
			var pod corev1.Pod
			if err := r.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
				return ctrl.Result{}, err
			}
			if pod.Status.PodIP == "" {
				// wait on pod ip
				log.Info("waiting on pod ip", "trace", trace.Name, "pod", pod.Name)
				return ctrl.Result{Requeue: true}, nil
			}
			log.Info("adding trace", "pod", pod.Name)
			trace.Target = pod.Status.PodIP
			yml, err := trace.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		case "service":
			log.Info("building trace configuration for service", "trace", trace.Name)
			var service corev1.Service
			if err := r.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding trace", "service", service.Name)
			target := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, trace.Port)
			trace.Target = target
			yml, err := trace.Yaml()
			if err != nil {
				return ctrl.Result{}, err
			}
			configData += yml
		case "ingress":
			log.Info("building trace configuration for ingress", "trace", trace.Name)
			var ingress networkingv1.Ingress
			if err := r.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding trace", "ingress", ingress.Name)
			for _, rule := range ingress.Spec.Rules {
				log.Info("adding trace rule", "host", rule.Host)
				trace.Target = rule.Host
				yml, err := trace.Yaml()
				if err != nil {
					return ctrl.Result{}, err
				}
				configData += yml
			}
		default:
			return ctrl.Result{}, fmt.Errorf("invalid trace kind %s", trace.Kind)
		}
	}

	//updateID := generateUpdateID(updateTasks)
	updateID := generateUpdateID(configData)
	log.Info("checking update", "updateID", updateID, "current", task.Status.UpdateID)

	if task.Status.UpdateID != updateID {
		log.Info("update required", "task", task.Name)
		task.Status.UpdateID = updateID
		task.Status.DeployNeeded = true

		log.Info("config update", "config", configData)
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

	log.Info("update and deployment complete", "task", task.Name)
	return ctrl.Result{}, nil
}

func generateUpdateID(configData string) string {
	h := sha1.New()
	h.Write([]byte(configData))
	return hex.EncodeToString(h.Sum(nil))
}
