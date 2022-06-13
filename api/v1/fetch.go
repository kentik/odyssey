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

package v1

import (
	"bytes"
	"fmt"
	"text/template"
)

const (
	fetchYamlTemplate = `
  - fetch:
      target: {{.Target}}
      port: {{.Port}}
      method: {{.Method}}
      insecure: {{.IgnoreTLSErrors}}
      period: {{.Period}}
      expiry: {{.Expiry}}
    network: ipv4
`
)

type Fetch struct {
	// Service is the name of the service for the check
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// Port is the port to use for the check
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Required
	Port int `json:"port"`
	// TLS is a bool to use HTTPS for the check
	// +optional
	TLS bool `json:"tls"`
	// Target is the target for the check
	// +kubebuilder:validation:Required
	Target string `json:"target"`
	// Method is the http method for the check
	// +kubebuilder:validation:Enum=get;head;post;GET;HEAD;POST
	// +kubebuilder:default=`get`
	// +kubebuilder:validation:Required
	Method string `json:"method"`
	// Period is the interval for which the server to run the check
	// +kubebuilder:default=`60s`
	// +optional
	Period string `json:"period"`
	// Expiry is the timeout for the check
	// +kubebuilder:default=`5s`
	// +optional
	Expiry string `json:"expiry"`
	// IgnoreTLSErrors is a bool to specify to ignore TLS errors
	// +optional
	IgnoreTLSErrors bool `json:"ignoreTLSErrors"`
}

func (f *Fetch) ID() string {
	return fmt.Sprintf("%s-%d-%s", f.Service, f.Port, f.Target)
}

func (f *Fetch) Yaml() (string, error) {
	t, err := template.New("fetch").Parse(fetchYamlTemplate)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	if err := t.Execute(&buf, f); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}
