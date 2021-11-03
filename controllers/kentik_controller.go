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
	"errors"
	"fmt"
	"strings"
	"time"

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	"github.com/kentik/odyssey/pkg/synthetics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type KentikReconciler struct {
	reconciler     *SyntheticTaskReconciler
	kentikEmail    string
	kentikAPIToken string
}

func NewKentikReconciler(r *SyntheticTaskReconciler, kentikEmail, kentikAPIToken string) *KentikReconciler {
	return &KentikReconciler{
		reconciler:     r,
		kentikEmail:    kentikEmail,
		kentikAPIToken: kentikAPIToken,
	}
}

func (r *KentikReconciler) Reconcile(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask) (ctrl.Result, error) {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "kentik")

	log.Info("checking agent deployment", "name", task.Name, "namespace", task.Namespace)
	// ksynth agent deployment
	currentAgentDeployment := appsv1.Deployment{}
	if err := r.reconciler.Get(ctx, types.NamespacedName{Name: getAgentDeploymentName(task), Namespace: task.Namespace}, &currentAgentDeployment); err != nil {
		// no deployment found; create
		if apierrors.IsNotFound(err) {
			log.Info("creating agent deployment")
			tld := "com"
			if strings.ToLower(task.Spec.KentikRegion) == "eu" {
				tld = "eu"
			}
			serverEndpoint := fmt.Sprintf("https://api.kentik.%s", tld)
			deployment := r.reconciler.getAgentDeployment(task, serverEndpoint)
			ctrl.SetControllerReference(task, deployment, r.reconciler.Scheme)
			if err := r.reconciler.Create(ctx, deployment); err != nil {
				return ctrl.Result{}, err
			}
			// deployed; requeue and wait
			return ctrl.Result{Requeue: true}, nil
		}

		log.Error(err, "error getting agent deployment")
		return ctrl.Result{}, err
	}

	// get agent pods
	agentPods, err := r.reconciler.getAgentPods(ctx, task)
	if err != nil {
		// wait on agent pods if nil
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken)

	// finalizers
	for _, pod := range agentPods {
		// deletion / finalizer
		if pod.ObjectMeta.DeletionTimestamp.IsZero() {
			// add finalizer if missing
			if !contains(pod.GetFinalizers(), finalizerName) {
				controllerutil.AddFinalizer(&pod, finalizerName)
				if err := r.reconciler.Update(ctx, &pod); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	// register and authorize agents
	for _, pod := range agentPods {
		agent, err := synthClient.GetAgent(ctx, pod.Name)
		if err != nil {
			if errors.Is(err, synthetics.ErrAgentNotFound) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}

		if agent.Status != synthetics.AgentStatusOK {
			// HACK:
			time.Sleep(time.Second * 15)
			log.Info("authorizing agent", "id", agent.ID)
			// TODO: check if already authorized and skip

			// TODO: auth agent id with site
			if err := synthClient.AuthorizeAgent(ctx, agent.ID, task.Spec.KentikSite); err != nil {
				return ctrl.Result{}, err
			}
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[agentPodAnnotationAgentID] = agent.ID
			if err := r.reconciler.Update(ctx, &pod); err != nil {
				return ctrl.Result{}, err
			}

			log.Info("agent authorized", "id", agent.ID)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// fetch tasks
	fetchTasksCreated, err := r.createFetchTasks(ctx, req, task, agentPods)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("fetchTasks", "status", fetchTasksCreated)
	if fetchTasksCreated {
		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: create tests via API and assign to agents
	testAgentIDs := []string{}
	for _, pod := range agentPods {
		if v, ok := pod.Annotations[agentPodAnnotationAgentID]; ok {
			testAgentIDs = append(testAgentIDs, v)
		}
	}

	// build and update config for server
	log.Info("checking tasks")
	//testIDs := []string{}

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
		case "service":
			log.Info("building ping configuration for service", "ping", ping.Name)
			var service corev1.Service
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding ping", "service", service.Name)
			ping.Target = service.Spec.ClusterIP
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
		case "service":
			log.Info("building trace configuration for service", "trace", trace.Name)
			var service corev1.Service
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("adding trace", "service", service.Name)
			target := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, trace.Port)
			trace.Target = target
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
			}
		default:
			return ctrl.Result{}, fmt.Errorf("invalid trace kind %s", trace.Kind)
		}
	}

	//updateID := generateUpdateID(testIDs)
	//log.Info("checking update", "updateID", updateID, "current", task.Status.UpdateID)

	//if task.Status.UpdateID != updateID {
	//	log.Info("update required", "task", task.Name)
	//	task.Status.UpdateID = updateID
	//	task.Status.DeployNeeded = true

	//	log.Info("config update", "config", configData)
	//	if err := r.reconciler.Status().Update(ctx, task); err != nil {
	//		return ctrl.Result{}, err
	//	}

	//	return ctrl.Result{Requeue: true}, nil
	//}

	//if task.Status.DeployNeeded {
	//	log.Info("deploy needed", "task", task.Name)
	//	podList := &corev1.PodList{}
	//	if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(serverLabels(task.Name))); err != nil {
	//		return ctrl.Result{}, err
	//	}

	//	for _, pod := range podList.Items {
	//		log.Info("deleting pod", "pod", pod.Name)
	//		if err := r.reconciler.Delete(ctx, &pod); err != nil {
	//			if !apierrors.IsNotFound(err) {
	//				return ctrl.Result{}, err
	//			}
	//		}
	//	}
	//	task.Status.DeployNeeded = false
	//	if err := r.reconciler.Status().Update(ctx, task); err != nil {
	//		return ctrl.Result{}, err
	//	}
	//}

	log.Info("update and deployment complete", "task", task.Name)
	return ctrl.Result{}, nil
}

func (r *KentikReconciler) Cleanup(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask) error {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "kentik")
	agentPods, err := r.reconciler.getAgentPods(ctx, task)
	if err != nil {
		// wait on agent pods if nil
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken)
	for _, pod := range agentPods {
		log.Info("checking delete pod finalizer", "pod", pod.Name)
		if contains(pod.GetFinalizers(), finalizerName) {
			log.Info("handling finalizer", "pod", pod.Name)
			if id, exists := pod.Annotations[agentPodAnnotationAgentID]; exists {
				// delete agent if exists
				if err := synthClient.DeleteAgent(ctx, id); err != nil {
					return err
				}

				log.Info("deleting agent", "id", id)
			}

			controllerutil.RemoveFinalizer(&pod, finalizerName)
			if err := r.reconciler.Update(ctx, &pod); err != nil {
				return err
			}
			log.Info("removed finalizer", "pod", pod.Name)
		}
	}

	return nil
}

func (r *KentikReconciler) createFetchTasks(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask, agentPods []corev1.Pod) (bool, error) {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "kentik")
	log.Info("agentPods", "pods", fmt.Sprintf("%+v", agentPods))

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken)
	// TODO: get all Kentik Tests and check if already created

	tests, err := synthClient.Tests(ctx)
	if err != nil {
		return false, err
	}

	testLookup := map[string]*synthetics.Test{}
	for _, t := range tests {
		testLookup[t.Name] = t
	}

	// fetch
	created := false
	for _, fetch := range task.Spec.Fetch {
		var svc corev1.Service
		if err := r.reconciler.Get(ctx, types.NamespacedName{Name: fetch.Service, Namespace: req.NamespacedName.Namespace}, &svc); err != nil {
			return false, err
		}
		for _, pod := range agentPods {
			agentID, exists := pod.Annotations[agentPodAnnotationAgentID]
			if !exists {
				return false, fmt.Errorf("unable to get agent id from pod annotations for %s", pod.Name)
			}
			// TODO: check if exists
			testName := fmt.Sprintf("%s/%s-fetch-%d-%s", task.Namespace, svc.Name, fetch.Port, agentID)
			if _, exists := testLookup[testName]; exists {
				log.Info("test exists", "name", testName)
				continue
			}
			created = true

			log.Info("building fetch configuration for service", "service", svc.Name)
			scheme := "http"
			if fetch.TLS {
				scheme = "https"
			}
			serviceEndpoint := fmt.Sprintf("%s://%s:%d%s", scheme, svc.Spec.ClusterIP, fetch.Port, fetch.Target)
			if err := synthClient.CreateTest(ctx, &synthetics.Test{
				Name: testName,
				Type: synthetics.TestTypeURL,
				Settings: &synthetics.TestSettings{
					RollupLevel: 60,
					Tasks:       []string{synthetics.TestTaskHTTP},
					URL: &synthetics.URLTest{
						Target: serviceEndpoint,
					},
					AgentIDs: []string{agentID},
				},
			}); err != nil {
				return false, err
			}
			log.V(0).Info("created fetch test", "name", testName)
		}
	}

	return created, nil
}

func contains(list []string, s string) bool {
	for _, i := range list {
		if strings.ToLower(i) == strings.ToLower(s) {
			return true
		}
	}

	return false
}
