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

package synthetics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

const (
	// Agent Status
	AgentStatusOK = "AGENT_STATUS_OK"
	IPFamilyV4    = "IP_FAMILY_V4"
)

var (
	ErrAgentNotFound = errors.New("agent not found")
)

type agentResponse struct {
	Agents []*Agent `json:"agents,omitempty"`
}

type authorizeAgent struct {
	Alias    string `json:"alias"`
	Family   string `json:"family"`
	Status   string `json:"status"`
	SiteID   string `json:"siteId"`
	SiteName string `json:"siteName"`
}
type agentAuthorizeRequest struct {
	Agent *authorizeAgent `json:"agent"`
}

type Agent struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Type     string `json:"type,omitempty"`
	OS       string `json:"os,omitempty"`
	IP       string `json:"ip,omitempty"`
	SiteID   string `json:"site_id,omitempty"`
	SiteName string `json:"site_name,omitempty"`
	Version  string `json:"version,omitempty"`
}

func (c *Client) Agents(ctx context.Context) ([]*Agent, error) {
	resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodGet, "/agents", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	agentResponse := &agentResponse{}
	if err := json.Unmarshal(data, &agentResponse); err != nil {
		return nil, err
	}

	return agentResponse.Agents, nil
}

func (c *Client) GetAgent(ctx context.Context, name string) (*Agent, error) {
	agents, err := c.Agents(ctx)
	if err != nil {
		return nil, err
	}

	for _, agent := range agents {
		if strings.ToLower(agent.Alias) == strings.ToLower(name) {
			return agent, nil
		}
	}

	return nil, errors.Wrap(ErrAgentNotFound, name)
}

func (c *Client) AuthorizeAgent(ctx context.Context, agentID, agentName, siteID string) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(agentAuthorizeRequest{
		Agent: &authorizeAgent{
			Alias:  agentName,
			Family: IPFamilyV4,
			Status: AgentStatusOK,
			SiteID: siteID,
		},
	}); err != nil {
		return err
	}

	resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodPut, fmt.Sprintf("/agents/%s", agentID), buf)
	if err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return errors.New(string(errData))
	}

	defer resp.Body.Close()

	return nil
}

func (c *Client) DeleteAgent(ctx context.Context, agentID string) error {
	if resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodDelete, fmt.Sprintf("/agents/%s", agentID), nil); err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(errData))
	}

	return nil
}
