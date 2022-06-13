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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	syntheticsv1 "github.com/kentik/odyssey/api/v1"
)

const (
	defaultSyntheticAgentImage          = "docker.io/kentik/ksynth:1.3.0"
	ownerKey                            = ".metadata.controller"
	agentName                           = "synthetics-agent"
	agentLabel                          = "kentiklabs.synthetics.agent"
	agentConfigMapName                  = "agent-config.yml"
	agentDeploymentConfigVolumeName     = "config"
	agentDeploymentContainerName        = "synthetics-agent"
	finalizerName                       = "com.kentiklabs.synthetics/finalizer"
	agentApiHostEnvVar                  = "KENTIK_API_HOST"
	agentKentikCompanyEnvVar            = "KENTIK_COMPANY"
	agentKentikSiteEnvVar               = "KENTIK_SITE"
	agentKentikRegionEnvVar             = "KENTIK_REGION"
	agentKentikAgentUpdateEnvVar        = "AGENT_UPDATE"
	agentKentikAgentAgentIdentityEnvVar = "AGENT_IDENTITY"
	agentPodAnnotationAgentID           = "com.kentiklabs.synthetics/agentID"
	taskAnnotationTestIDs               = "com.kentiklabs.synthetics/testIDs"
)

var (
	defaultAgentCommand = []string{"/opt/kentik/ksynth/ksynth", "agent", "-vv", "-4"}
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
	syntheticTask *syntheticsv1.SyntheticTask
	tasks         []updateTask
}

// SyntheticTaskReconciler reconciles a SyntheticTask object
type SyntheticTaskReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	KentikEmail    string
	KentikAPIToken string
	tasks          map[string]interface{}
	updateCh       chan *updateConfig
	kentikCompany  string
}

type Reconciler interface {
	Reconcile(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask) (ctrl.Result, error)
	Cleanup(ctx context.Context, req ctrl.Request, task *syntheticsv1.SyntheticTask) error
}

//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=synthetics.kentiklabs.com,resources=synthetictasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;create;update;watch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
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

	var reconciler Reconciler
	// default to synsrv
	reconciler = NewLocalReconciler(r)
	// check for Kentik
	if task.Spec.KentikSite != "" {
		// check for valid controller config
		if r.KentikEmail == "" || r.KentikAPIToken == "" {
			return ctrl.Result{}, fmt.Errorf("kentik-email and kentik-api-token must be specified on the controller in order to process kentik integrated tasks")
		}

		reconciler = NewKentikReconciler(r, r.KentikEmail, r.KentikAPIToken)
	}

	// task finalizer to allow for dependent object cleanup
	taskFinalizers := task.GetFinalizers()
	if task.ObjectMeta.DeletionTimestamp.IsZero() {
		// add finalizer if missing
		if !contains(taskFinalizers, finalizerName) {
			controllerutil.AddFinalizer(&task, finalizerName)
			if err := r.Update(ctx, &task); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else { // deleted
		log.Info("checking delete finalizer", "task", task.Name)
		if contains(taskFinalizers, finalizerName) {
			if err := reconciler.Cleanup(ctx, req, &task); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&task, finalizerName)
			if err := r.Update(ctx, &task); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("removed finalizer", "task", task.Name)
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return reconciler.Reconcile(ctx, req, &task)
}
