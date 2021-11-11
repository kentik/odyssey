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
	"bytes"
	"fmt"
	"text/template"
)

const (
	pingYamlTemplate = `
  - ping:
      target: {{.Target}}
      count: {{.Count}}
      period: {{.Period}}
      delay: {{.Delay}}
      expiry: {{.Expiry}}
    ipv4: true
    ipv6: false
`
)

type Ping struct {
	// Kind is the k8s object for the check
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=deployment;pod;service;ingress
	Kind string `json:"kind"`
	// Name is the name of the k8s object to check
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Count is the number of tries to use for the check
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	// +optional
	Count int `json:"count"`
	// Period is the interval for which the server to run the check
	// +kubebuilder:default=`60s`
	// +optional
	Period string `json:"period"`
	// Delay is the duration to wait between checks
	// +kubebuilder:validation:Pattern=`^[0-9]+ms`
	// +kubebuilder:default=`0ms`
	// +optional
	Delay string `json:"delay"`
	// Expiry is the timeout for the check
	// +kubebuilder:default=`2s`
	// +optional
	Expiry string `json:"expiry"`

	// Target is used in the yaml definition but not exposed to the user
	// +optional
	Target string `json:"-"`
}

func (p *Ping) ID() string {
	return fmt.Sprintf("%s-%s-%d", p.Kind, p.Name, p.Count)
}

func (p *Ping) Yaml() (string, error) {
	tmpl, err := template.New("ping").Parse(pingYamlTemplate)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}
