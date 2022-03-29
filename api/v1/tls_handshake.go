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
	tlsHandshakeYamlTemplate = `
  - shake:
      target: {{.Target}}
      port: {{.Port}}
      period: {{.Period}}
      expiry: {{.Expiry}}
    ipv4: true
    ipv6: false
`
)

type TLSHandshake struct {
	// Ingress is the name of the ingress for the check
	// +kubebuilder:validation:Required
	Ingress string `json:"ingress"`
	// Port is the port to use for the check
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=443
	// +optional
	Port int `json:"port"`
	// Period is the interval for which the server to run the check
	// +kubebuilder:default=`10s`
	// +optional
	Period string `json:"period"`
	// Expiry is the timeout for the check
	// +kubebuilder:default=`5s`
	// +optional
	Expiry string `json:"expiry"`

	// Target is used in the yaml definition but not exposed to the user
	// +optional
	Target string `json:"-"`
}

func (t *TLSHandshake) ID() string {
	return fmt.Sprintf("%s-%d", t.Ingress, t.Port)
}

func (t *TLSHandshake) Yaml() (string, error) {
	tmpl, err := template.New("tlsHandshake").Parse(tlsHandshakeYamlTemplate)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, t); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}
