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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SyntheticTaskSpec defines the desired state of SyntheticTask
type SyntheticTaskSpec struct {
	// ServerImage is an optional override server image
	// +optional
	ServerImage string `json:"server_image,omitempty"`
	// ServerCommand is an optional override command for the server
	// +optional
	ServerCommand []string `json:"server_command,omitempty"`
	// AgentImage is an optional override agent image
	// +optional
	AgentImage string `json:"agent_image,omitempty"`
	// AgentCommand is an optional override command for the agent
	// +optional
	AgentCommand []string `json:"agent_command,omitempty"`
	// Fetch is a list of fetch checks
	// +optional
	Fetch []Fetch `json:"fetch,omitempty"`
}

// SyntheticTaskStatus defines the observed state of SyntheticTask
type SyntheticTaskStatus struct {
	// UpdateID is the current updateID for the server
	// +optional
	UpdateID string `json:"update_id"`
	// DeployNeeded indicates the server needs re-deployed for changes
	// +optional
	DeployNeeded bool `json:"deploy_needed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SyntheticTask is the Schema for the synthetictasks API
type SyntheticTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyntheticTaskSpec   `json:"spec,omitempty"`
	Status SyntheticTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SyntheticTaskList contains a list of SyntheticTask
type SyntheticTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyntheticTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyntheticTask{}, &SyntheticTaskList{})
}
