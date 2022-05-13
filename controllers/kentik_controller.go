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
	"errors"
	"fmt"
	"strings"
	"time"

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	"github.com/kentik/odyssey/pkg/synthetics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
			deployment, err := r.reconciler.getAgentDeployment(task, serverEndpoint, nil)
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

	if len(agentPods) != int(*currentAgentDeployment.Spec.Replicas) {
		log.Info(fmt.Sprintf("waiting on agent pods (%d/%d)", len(agentPods), *currentAgentDeployment.Spec.Replicas))
		return ctrl.Result{Requeue: true}, nil
	}

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken, r.reconciler.Log)

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
			log.Info("authorizing agent", "id", agent.ID)

			site, err := synthClient.GetSite(ctx, task.Spec.KentikSite)
			if err != nil {
				return ctrl.Result{}, err
			}

			// auth agent id with site
			if err := synthClient.AuthorizeAgent(ctx, agent.ID, pod.Name, fmt.Sprintf("%d", site.ID)); err != nil {
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
	if fetchTasksCreated {
		return ctrl.Result{Requeue: true}, nil
	}
	// ping
	pingTasksCreated, err := r.createPingTasks(ctx, req, task, agentPods)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pingTasksCreated {
		return ctrl.Result{Requeue: true}, nil
	}
	// trace
	traceTasksCreated, err := r.createTraceTasks(ctx, req, task, agentPods)
	if err != nil {
		return ctrl.Result{}, err
	}
	if traceTasksCreated {
		return ctrl.Result{Requeue: true}, nil
	}

	// build and update config for server

	//// tls handshake
	//for _, tlsHandshake := range task.Spec.TLSHandshake {
	//	var ingress networkingv1.Ingress
	//	if err := r.reconciler.Get(ctx, types.NamespacedName{Name: tlsHandshake.Ingress, Namespace: req.NamespacedName.Namespace}, &ingress); err != nil {
	//		return ctrl.Result{}, err
	//	}
	//	log.Info("building tls handshake configuration for ingress", "ingress", ingress.Name)
	//	log.V(0).Info("adding tls handshake", "shake", fmt.Sprintf("%+v", tlsHandshake))
	//	for _, rule := range ingress.Spec.Rules {
	//		log.Info("adding tls handshake rule", "host", rule.Host)
	//		tlsHandshake.Target = rule.Host
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

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken, r.reconciler.Log)
	for _, pod := range agentPods {
		log.Info("checking delete pod finalizer", "pod", pod.Name)
		if contains(pod.GetFinalizers(), finalizerName) {
			log.Info("handling finalizer", "pod", pod.Name)
			// delete tests
			v, ok := task.Annotations[taskAnnotationTestIDs]
			if ok {
				testIDs := strings.Split(v, ",")
				for _, testID := range testIDs {
					log.Info("deleting test", "id", testID)
					if err := synthClient.DeleteTest(ctx, testID); err != nil {
						return err
					}
				}
			}

			// cleanup agents
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

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken, r.reconciler.Log)
	// TODO: get all Kentik Tests and check if already created

	tests, err := synthClient.Tests(ctx)
	if err != nil {
		return false, err
	}

	testLookup := map[string]*synthetics.Test{}
	for _, t := range tests {
		testLookup[t.Name] = t
	}

	// get agentIDs
	agentIDs := []string{}
	for _, pod := range agentPods {
		agentID, exists := pod.Annotations[agentPodAnnotationAgentID]
		if !exists {
			return false, fmt.Errorf("unable to get agent id from pod annotations for %s", pod.Name)
		}
		agentIDs = append(agentIDs, agentID)
	}

	// fetch
	created := false
	for _, fetch := range task.Spec.Fetch {
		var svc corev1.Service
		if err := r.reconciler.Get(ctx, types.NamespacedName{Name: fetch.Service, Namespace: req.NamespacedName.Namespace}, &svc); err != nil {
			return false, err
		}
		// check if exists
		testName := fmt.Sprintf("%s/%s-fetch-%d", task.Namespace, svc.Name, fetch.Port)
		if _, exists := testLookup[testName]; exists {
			// check if agent matches; if not call UpdateTest with agent id
			log.Info("test exists", "name", testName)
			continue
		}
		created = true

		log.Info("building fetch configuration for service", "service", svc.Name)
		scheme := "http"
		if fetch.TLS {
			log.Info("using tls for fetch service", "service", svc.Name)
			log.Info("tls", "insecure", fetch.IgnoreTLSErrors)
			scheme = "https"
		}
		serviceEndpoint := fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d%s", scheme, svc.Name, task.Namespace, fetch.Port, fetch.Target)
		testInterval, err := time.ParseDuration(fetch.Period)
		if err != nil {
			return false, err
		}
		test, err := synthClient.CreateTest(ctx, &synthetics.Test{
			Name: testName,
			Type: synthetics.TestTypeURL,
			Settings: &synthetics.TestSettings{
				Period: int(testInterval.Seconds()),
				//RollupLevel: 60,
				Tasks: []string{synthetics.TestTaskHTTP},
				URL: &synthetics.URLTest{
					Target:          serviceEndpoint,
					Method:          fetch.Method,
					IgnoreTLSErrors: fetch.IgnoreTLSErrors,
					Timeout:         5000,
				},
				AgentIDs: agentIDs,
			},
		})
		if err != nil {
			return false, err
		}
		// update task annotations with test IDs
		if err := r.updateTaskTestIDs(ctx, task, test.ID); err != nil {
			return false, err
		}
		log.Info("created fetch test", "name", testName, "id", test.ID)
	}

	return created, nil
}

func (r *KentikReconciler) createPingTasks(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask, agentPods []corev1.Pod) (bool, error) {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "kentik")

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken, r.reconciler.Log)

	tests, err := synthClient.Tests(ctx)
	if err != nil {
		return false, err
	}

	testLookup := map[string]*synthetics.Test{}
	for _, t := range tests {
		testLookup[t.Name] = t
	}

	// get agentIDs
	agentIDs := []string{}
	for _, pod := range agentPods {
		agentID, exists := pod.Annotations[agentPodAnnotationAgentID]
		if !exists {
			return false, fmt.Errorf("unable to get agent id from pod annotations for %s", pod.Name)
		}
		agentIDs = append(agentIDs, agentID)
	}

	// ping
	created := false
	for _, ping := range task.Spec.Ping {
		// create test per agent
		testName := fmt.Sprintf("%s/ping-%s", task.Namespace, ping.Name)
		if _, exists := testLookup[testName]; exists {
			// check if agent matches; if not call UpdateTest with agent id
			log.Info("test exists", "name", testName)
			continue
		}
		created = true

		targetIPs := []string{}
		switch strings.ToLower(ping.Kind) {
		case "deployment":
			// check if exists
			var deploy appsv1.Deployment
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return false, err
			}
			podList := &corev1.PodList{}
			if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
				return false, err
			}
			existingPods := map[string]struct{}{}
			for _, pod := range podList.Items {
				log.Info("building ping configuration for deployment", "ping", ping.Name)
				if pod.Status.PodIP == "" {
					// wait on pod ip
					log.Info("waiting on pod ip", "ping", ping.Name, "pod", pod.Name)
					return true, nil
				}
				if _, exists := existingPods[pod.Status.PodIP]; exists {
					log.Info("skipping existing pod", "pod", pod.Name)
					continue
				}
				targetIPs = append(targetIPs, pod.Status.PodIP)
				log.Info("added target ip for ping", "deployment", deploy.Name, "pod", pod.Name, "ip", pod.Status.PodIP)
			}
		case "pod":
			log.Info("building ping configuration for pod", "ping", ping.Name)
			var pod corev1.Pod
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
				return false, err
			}
			if pod.Status.PodIP == "" {
				// wait on pod ip
				log.Info("waiting on pod ip", "ping", ping.Name, "pod", pod.Name)
				return true, err
			}
			targetIPs = append(targetIPs, pod.Status.PodIP)
			log.Info("added target ip for ping", "pod", pod.Name, "ip", pod.Status.PodIP)
		case "service":
			log.Info("building ping configuration for service", "ping", ping.Name)
			var service corev1.Service
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: ping.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return false, err
			}
			targetIPs = append(targetIPs, service.Spec.ClusterIP)
			log.Info("added target ip for ping", "service", service.Name, "ip", service.Spec.ClusterIP)
		default:
			return false, fmt.Errorf("invalid ping kind %s", ping.Kind)
		}

		pingPeriod, err := time.ParseDuration(ping.Period)
		if err != nil {
			return false, err
		}
		pingDelay, err := time.ParseDuration(ping.Delay)
		if err != nil {
			return false, err
		}

		if ping.Count <= 0 {
			ping.Count = 1
		}

		test, err := synthClient.CreateTest(ctx, &synthetics.Test{
			Name: testName,
			Type: synthetics.TestTypeIP,
			Settings: &synthetics.TestSettings{
				Period: int(pingPeriod.Seconds()),
				Tasks:  []string{synthetics.TestTaskPing},
				Ping: &synthetics.PingTest{
					Protocol: ping.Protocol,
					Port:     ping.Port,
					Count:    ping.Count,
					Delay:    int(pingDelay.Milliseconds()),
					Timeout:  ping.Timeout,
				},
				Trace: &synthetics.TraceTest{
					Protocol: ping.Protocol,
					Port:     ping.Port,
					Count:    ping.Count,
					Delay:    int(pingDelay.Milliseconds()),
					Limit:    5,
					Timeout:  ping.Timeout,
				},
				IP: &synthetics.TestIP{
					Targets: targetIPs,
				},
				AgentIDs: agentIDs,
			},
		})
		if err != nil {
			return false, err
		}

		if err := r.updateTaskTestIDs(ctx, task, test.ID); err != nil {
			return false, err
		}
		log.Info("created ping test", "name", testName, "id", test.ID)
	}

	return created, nil
}

func (r *KentikReconciler) createTraceTasks(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask, agentPods []corev1.Pod) (bool, error) {
	log := r.reconciler.Log.WithValues("name", req.NamespacedName, "namespace", req.Namespace, "controller", "kentik")

	synthClient := synthetics.NewClient(r.kentikEmail, r.kentikAPIToken, r.reconciler.Log)

	tests, err := synthClient.Tests(ctx)
	if err != nil {
		return false, err
	}

	testLookup := map[string]*synthetics.Test{}
	for _, t := range tests {
		testLookup[t.Name] = t
	}

	// get agentIDs
	agentIDs := []string{}
	for _, pod := range agentPods {
		agentID, exists := pod.Annotations[agentPodAnnotationAgentID]
		if !exists {
			return false, fmt.Errorf("unable to get agent id from pod annotations for %s", pod.Name)
		}
		agentIDs = append(agentIDs, agentID)
	}

	// trace
	created := false
	for _, trace := range task.Spec.Trace {
		log.Info("trace task", "trace", fmt.Sprintf("%+v", trace))
		// create test per agent
		testName := fmt.Sprintf("%s/trace-%s", task.Namespace, trace.Name)
		if _, exists := testLookup[testName]; exists {
			// check if agent matches; if not call UpdateTest with agent id
			log.Info("test exists", "name", testName)
			continue
		}
		created = true

		targetIPs := []string{}
		switch strings.ToLower(trace.Kind) {
		case "deployment":
			// check if exists
			var deploy appsv1.Deployment
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &deploy); err != nil {
				return false, err
			}
			podList := &corev1.PodList{}
			if err := r.reconciler.List(ctx, podList, client.InNamespace(task.Namespace), client.MatchingLabels(deploy.Spec.Selector.MatchLabels)); err != nil {
				return false, err
			}
			existingPods := map[string]struct{}{}
			for _, pod := range podList.Items {
				log.Info("building trace configuration for deployment", "trace", trace.Name)
				if pod.Status.PodIP == "" {
					// wait on pod ip
					log.Info("waiting on pod ip", "trace", trace.Name, "pod", pod.Name)
					return true, nil
				}
				if _, exists := existingPods[pod.Status.PodIP]; exists {
					log.Info("skipping existing pod", "pod", pod.Name)
					continue
				}
				targetIPs = append(targetIPs, pod.Status.PodIP)
				log.Info("added target ip for trace", "deployment", deploy.Name, "pod", pod.Name, "ip", pod.Status.PodIP)
			}
		case "pod":
			log.Info("building trace configuration for pod", "trace", trace.Name)
			var pod corev1.Pod
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &pod); err != nil {
				return false, err
			}
			if pod.Status.PodIP == "" {
				// wait on pod ip
				log.Info("waiting on pod ip", "trace", trace.Name, "pod", pod.Name)
				return true, err
			}
			targetIPs = append(targetIPs, pod.Status.PodIP)
			log.Info("added target ip for trace", "pod", pod.Name, "ip", pod.Status.PodIP)
		case "service":
			log.Info("building trace configuration for service", "trace", trace.Name)
			var service corev1.Service
			if err := r.reconciler.Get(ctx, types.NamespacedName{Name: trace.Name, Namespace: req.NamespacedName.Namespace}, &service); err != nil {
				return false, err
			}
			targetIPs = append(targetIPs, service.Spec.ClusterIP)
			log.Info("added target ip for trace", "service", service.Name, "ip", service.Spec.ClusterIP)
		default:
			return false, fmt.Errorf("invalid trace kind %s", trace.Kind)
		}

		traceDelay, err := time.ParseDuration(trace.Delay)
		if err != nil {
			return false, err
		}

		test, err := synthClient.CreateTest(ctx, &synthetics.Test{
			Name: testName,
			Type: synthetics.TestTypeIP,
			Settings: &synthetics.TestSettings{
				Tasks: []string{
					synthetics.TestTaskPing,
					synthetics.TestTaskTrace,
				},
				Ping: &synthetics.PingTest{
					Protocol: synthetics.TestProtocolTCP,
					Port:     trace.Port,
					Count:    trace.Count,
					Delay:    int(traceDelay.Milliseconds()),
					Timeout:  trace.Timeout,
				},
				Trace: &synthetics.TraceTest{
					Protocol: synthetics.TestProtocolTCP,
					Port:     trace.Port,
					Count:    trace.Count,
					Limit:    trace.Limit,
					Delay:    int(traceDelay.Milliseconds()),
					Timeout:  trace.Timeout,
				},
				IP: &synthetics.TestIP{
					Targets: targetIPs,
				},
				AgentIDs: agentIDs,
			},
		})
		if err != nil {
			return false, err
		}

		if err := r.updateTaskTestIDs(ctx, task, test.ID); err != nil {
			return false, err
		}
		log.Info("created trace test", "name", testName, "id", test.ID)
	}

	return created, nil
}

func (r *KentikReconciler) updateTaskTestIDs(ctx context.Context, task *syntheticsv1.SyntheticTask, testID string) error {

	// update task annotations with test IDs
	if task.Annotations == nil {
		task.Annotations = map[string]string{}
	}
	v, _ := task.Annotations[taskAnnotationTestIDs]
	testIDs := []string{}
	if v != "" {
		testIDs = strings.Split(v, ",")
	}
	testIDs = append(testIDs, testID)
	task.Annotations[taskAnnotationTestIDs] = strings.Join(testIDs, ",")
	if err := r.reconciler.Update(ctx, task); err != nil {
		return err
	}

	return nil
}

func contains(list []string, s string) bool {
	for _, i := range list {
		if strings.ToLower(i) == strings.ToLower(s) {
			return true
		}
	}

	return false
}
