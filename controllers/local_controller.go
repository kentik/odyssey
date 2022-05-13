/*
Copyright 2022 KentikLabs

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

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalReconciler struct {
	reconciler *SyntheticTaskReconciler
}

func NewLocalReconciler(r *SyntheticTaskReconciler) *LocalReconciler {
	return &LocalReconciler{
		reconciler: r,
	}
}

func (r *LocalReconciler) Reconcile(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask) (ctrl.Result, error) {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "local")

	currentAgentConfigMap := corev1.ConfigMap{}
	if err := r.reconciler.Get(ctx, types.NamespacedName{Name: getAgentConfigMapName(task), Namespace: task.Namespace}, &currentAgentConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		log.Info("creating agent configmap")
		configMap := r.reconciler.getAgentConfigMap(task, baseServerConfig)
		ctrl.SetControllerReference(task, configMap, r.reconciler.Scheme)
		if err := r.reconciler.Create(ctx, configMap); err != nil {
			log.Error(err, "error getting agent configmap")
			return ctrl.Result{}, err
		}
		// created; requeue and wait
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("checking agent deployment", "name", task.Name, "namespace", task.Namespace)
	// ksynth agent deployment
	currentAgentDeployment := appsv1.Deployment{}
	if err := r.reconciler.Get(ctx, types.NamespacedName{Name: getAgentDeploymentName(task), Namespace: task.Namespace}, &currentAgentDeployment); err != nil {
		// no deployment found; create
		if apierrors.IsNotFound(err) {
			log.Info("creating agent deployment")

			r.reconciler.Log.Info("configuring for local export", "task", task.Name)

			deployment, err := r.reconciler.getAgentDeployment(task, "", &currentAgentConfigMap)
			if err != nil {
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(task, deployment, r.reconciler.Scheme)
			if err := r.reconciler.Create(ctx, deployment); err != nil {
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
		if err := r.reconciler.Get(ctx, types.NamespacedName{Name: fetch.Service, Namespace: req.NamespacedName.Namespace}, &svc); err != nil {
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
		if err := r.reconciler.Get(ctx, types.NamespacedName{Name: tlsHandshake.Ingress, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return ctrl.Result{}, err
			}
			podList := &corev1.PodList{}
			if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return ctrl.Result{}, err
			}
			podList := &corev1.PodList{}
			if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
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
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
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

	updateID := generateUpdateID(configData)
	log.Info("checking update", "updateID", updateID, "current", task.Status.UpdateID)

	if task.Status.UpdateID != updateID {
		log.Info("update required", "task", task.Name)
		task.Status.UpdateID = updateID
		task.Status.DeployNeeded = true

		log.Info("config update", "config", configData)
		currentAgentConfigMap.Data[agentConfigMapName] = configData
		if err := r.reconciler.Update(ctx, &currentAgentConfigMap); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconciler.Status().Update(ctx, task); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if task.Status.DeployNeeded {
		log.Info("deploy needed", "task", task.Name)
		podList := &corev1.PodList{}
		if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(agentLabels(task.Name))); err != nil {
			return ctrl.Result{}, err
		}

		for _, pod := range podList.Items {
			log.Info("deleting pod", "pod", pod.Name)
			if err := r.reconciler.Delete(ctx, &pod); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
		task.Status.DeployNeeded = false
		if err := r.reconciler.Status().Update(ctx, task); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("update and deployment complete", "task", task.Name)
	return ctrl.Result{}, nil
}

func (r *LocalReconciler) Cleanup(_ context.Context, _ ctrl.Request, _ *syntheticsv1.SyntheticTask) error {
	return nil
}

func generateUpdateID(configData string) string {
	h := sha1.New()
	h.Write([]byte(configData))
	return hex.EncodeToString(h.Sum(nil))
}
