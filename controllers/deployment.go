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
	"fmt"
	"time"

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *SyntheticTaskReconciler) getServerConfigMap(t *syntheticsv1.SyntheticTask, data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServerConfigMapName(t),
			Namespace: t.Namespace,
		},
		Data: map[string]string{
			serverConfigMapName: data,
		},
	}
}

func (r *SyntheticTaskReconciler) getServerDeployment(t *syntheticsv1.SyntheticTask, configMap *corev1.ConfigMap) *appsv1.Deployment {
	replicas := int32(1)
	labels := serverLabels(t.Name)
	image := t.Spec.ServerImage
	if image == "" {
		image = defaultSyntheticServerImage
	}
	serverCmd := []string{
		"synsrv",
		"-v",
		"server",
		"-b",
		fmt.Sprintf("0.0.0.0:%d", serverPort),
		"-c",
		fmt.Sprintf("/etc/kentik/%s", serverConfigMapName),
	}
	if len(t.Spec.ServerCommand) > 0 {
		serverCmd = t.Spec.ServerCommand
	}
	serverContainer := corev1.Container{
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Name:            serverDeploymentContainerName,
		Command:         serverCmd,
		Ports: []corev1.ContainerPort{
			{
				Name:          serverPortName,
				ContainerPort: serverPort,
			},
		},
	}
	podSpec := corev1.PodSpec{}
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
		serverContainer.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      serverDeploymentConfigVolumeName,
				ReadOnly:  true,
				MountPath: "/etc/kentik",
			},
		}
	}
	podSpec.Containers = []corev1.Container{serverContainer}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServerDeploymentName(t),
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

func (s *SyntheticTaskReconciler) getServerService(t *syntheticsv1.SyntheticTask) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getServerServiceName(t),
			Namespace: t.Namespace,
			Labels:    serverLabels(t.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: serverLabels(t.Name),
			Ports: []corev1.ServicePort{
				{
					Name:     serverPortName,
					Protocol: corev1.ProtocolTCP,
					Port:     serverPort,
				},
			},
		},
	}
}

func (r *SyntheticTaskReconciler) getAgentDeployment(t *syntheticsv1.SyntheticTask, serverEndpoint string) *appsv1.Deployment {
	replicas := int32(1)
	labels := agentLabels(t.Name)
	image := t.Spec.AgentImage
	if image == "" {
		image = defaultSyntheticAgentImage
	}
	cmd := defaultAgentCommand
	if v := t.Spec.InfluxDB; v != nil {
		influxEndpoint := fmt.Sprintf("%s?org=%s&bucket=%s", v.Endpoint, v.Organization, v.Bucket)
		r.Log.Info("configuring influxdb output", "target", influxEndpoint)
		cmd = append(cmd, "--output", fmt.Sprintf("influx=%s,username=%s,password=%s,token=%s", influxEndpoint, v.Username, v.Password, v.Token))
	}
	r.Log.Info("agent deployment", "endpoint", serverEndpoint)
	env := []corev1.EnvVar{
		{
			Name:  agentApiHostEnvVar,
			Value: serverEndpoint,
		},
		{
			Name:  agentKentikAgentUpdateEnvVar,
			Value: "true",
		},
		{
			Name:  agentKentikAgentAgentIdentityEnvVar,
			Value: fmt.Sprintf("%d", time.Now().UnixNano()),
		},
	}
	if v := t.Spec.KentikCompany; v != "" {
		env = append(env, corev1.EnvVar{
			Name:  agentKentikCompanyEnvVar,
			Value: v,
		})
	}
	if v := t.Spec.KentikSite; v != "" {
		env = append(env, corev1.EnvVar{
			Name:  agentKentikSiteEnvVar,
			Value: v,
		})
	}
	if v := t.Spec.KentikRegion; v != "" {
		env = append(env, corev1.EnvVar{
			Name:  agentKentikRegionEnvVar,
			Value: v,
		})
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

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			agentContainer,
		},
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

	return deploy
}

func getServerConfigMapName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, serverName)
}

func getServerDeploymentName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, serverName)
}

func getServerServiceName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, serverName)
}

func getAgentDeploymentName(task *syntheticsv1.SyntheticTask) string {
	return fmt.Sprintf("%s-%s", task.Name, agentName)
}

func serverLabels(name string) map[string]string {
	return map[string]string{serverLabel: name}
}

func agentLabels(name string) map[string]string {
	return map[string]string{agentLabel: name}

}
