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

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
