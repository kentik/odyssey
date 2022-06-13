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
	"path"
	"time"

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	"github.com/kentik/odyssey/pkg/synthetics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *SyntheticTaskReconciler) getAgentConfigMap(t *syntheticsv1.SyntheticTask, data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getAgentConfigMapName(t),
			Namespace: t.Namespace,
		},
		Data: map[string]string{
			agentConfigMapName: data,
		},
	}
}

func (r *SyntheticTaskReconciler) getAgentDeployment(t *syntheticsv1.SyntheticTask, serverEndpoint string, configMap *corev1.ConfigMap) (*appsv1.Deployment, error) {
	replicas := int32(1)
	labels := agentLabels(t.Name)
	image := t.Spec.AgentImage
	if image == "" {
		image = defaultSyntheticAgentImage
	}
	cmd := defaultAgentCommand
	if v := t.Spec.InfluxDB; v != nil {
		influxEndpoint := fmt.Sprintf("%s?org=%s&bucket=%s&precision=ns", v.Endpoint, v.Organization, v.Bucket)
		r.Log.Info("configuring influxdb output", "target", influxEndpoint)
		cmd = append(cmd, "--output", fmt.Sprintf("influx,endpoint=%s,token=%s", influxEndpoint, v.Token))
	}
	r.Log.Info("agent deployment", "endpoint", serverEndpoint)

	env := []corev1.EnvVar{
		{
			Name:  agentKentikAgentUpdateEnvVar,
			Value: "true",
		},
		{
			Name:  agentKentikAgentAgentIdentityEnvVar,
			Value: fmt.Sprintf("%d", time.Now().UnixNano()),
		},
	}

	// kentik specific deploy
	if t.Spec.KentikSite != "" {
		synthClient := synthetics.NewClient(r.KentikEmail, r.KentikAPIToken, r.Log)
		env = append(env,
			corev1.EnvVar{
				Name:  agentApiHostEnvVar,
				Value: serverEndpoint,
			},
		)
		site, err := synthClient.GetSite(context.Background(), t.Spec.KentikSite)
		if err != nil {
			return nil, err
		}
		r.Log.Info("using kentik company", "id", site.CompanyID)

		env = append(env, corev1.EnvVar{
			Name:  agentKentikCompanyEnvVar,
			Value: fmt.Sprintf("%d", site.CompanyID),
		})

		r.Log.Info("using kentik site", "id", site.ID)
		env = append(env, corev1.EnvVar{
			Name:  agentKentikSiteEnvVar,
			Value: fmt.Sprintf("%d", site.ID),
		})

		if v := t.Spec.KentikRegion; v != "" {
			env = append(env, corev1.EnvVar{
				Name:  agentKentikRegionEnvVar,
				Value: v,
			})
		}
	}

	agentContainer := corev1.Container{
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Name:            agentDeploymentContainerName,
		Command:         cmd,
		Env:             env,
	}
	if len(t.Spec.AgentCommand) > 0 {
		agentContainer.Command = t.Spec.AgentCommand
	}

	volumes := []corev1.Volume{}

	// agent with configmap
	if configMap != nil {
		configMountPath := "/etc/kentik"
		volumes = []corev1.Volume{
			{
				Name: agentDeploymentConfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{

							Name: configMap.Name,
						},
					},
				},
			},
		}
		agentContainer.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      agentDeploymentConfigVolumeName,
				ReadOnly:  true,
				MountPath: configMountPath,
			},
		}
		agentContainer.Command = append(agentContainer.Command, []string{"--config", path.Join(configMountPath, agentConfigMapName)}...)
		r.Log.Info("updated agent deploy for local config map", "config", configMap.Name, "agentCommand", agentContainer.Command)
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			agentContainer,
		},
		Volumes: volumes,
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getAgentDeploymentName(t),
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

	return deploy, nil
}

func getAgentConfigMapName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, agentName)
}

func getAgentDeploymentName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, agentName)
}

func agentLabels(name string) map[string]string {
	return map[string]string{agentLabel: name}

}
