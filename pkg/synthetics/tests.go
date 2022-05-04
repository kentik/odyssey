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
	// Test Types
	TestTypeURL = "url"
	TestTypeIP  = "ip"
	// Test Status
	TestStatusActive = "TEST_STATUS_ACTIVE"
	TestStatusPaused = "TEST_STATUS_PAUSED"
	// Test Tasks
	TestTaskHTTP  = "http"
	TestTaskTrace = "traceroute"
	TestTaskPing  = "ping"
	// Test IP Family
	TestIPFamilyV4   = "IP_FAMILY_V4"
	TestIPFamilyV6   = "IP_FAMILY_V6"
	TestIPFamilyDual = "IP_FAMILY_DUAL"
	// Test Protocol
	TestProtocolTCP  = "tcp"
	TestProtocolUDP  = "udp"
	TestProtocolICMP = "icmp"

	// API minimum values
	fetchPeriodMinimum = 60
	tracePeriodMinimum = 60
	pingPeriodMinimum  = 60
)

type testCreateRequest struct {
	Test *Test `json:"test"`
}

type testCreateResponse struct {
	Test *Test `json:"test"`
}

type testsResponse struct {
	Tests []*Test `json:"tests"`
}

type testUpdateRequest struct {
	Test *Test  `json:"test"`
	Mask string `json:"mask"`
}

type Test struct {
	ID       string        `json:"id,omitempty"`
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Status   string        `json:"status"`
	Settings *TestSettings `json:"settings"`
}

type TestSettings struct {
	Hostname       *TestHostname       `json:"hostname,omitempty"`
	IP             *TestIP             `json:"ip,omitempty"`
	Agent          *TestAgent          `json:"agent,omitempty"`
	Flow           *TestFlow           `json:"flow,omitempty"`
	Site           *TestSite           `json:"site,omitempty"`
	Tag            *TestTag            `json:"tag,omitempty"`
	DNS            *DNSTest            `json:"dns,omitempty"`
	URL            *URLTest            `json:"url,omitempty"`
	NetworkGrid    *TestNetworkGrid    `json:"networkGrid,omitempty"`
	PageLoad       *PageLoadTest       `json:"pageLoad,omitempty"`
	DNSGrid        *TestDNSGrid        `json:"dnsGrid,omitempty"`
	AgentIDs       []string            `json:"agentIds,omitempty"`
	Period         int                 `json:"period,omitempty"`
	Tasks          []string            `json:"tasks,omitempty"`
	HealthSettings *TestHealthSettings `json:"healthSettings,omitempty"`
	Ping           *PingTest           `json:"ping,omitempty"`
	Trace          *TraceTest          `json:"trace,omitempty"`
	Port           int                 `json:"port,omitempty"`
	Protocol       string              `json:"protocol,omitempty"`
	Family         string              `json:"family,omitempty"`
	Servers        []string            `json:"servers,omitempty"`
	UseLocalIP     bool                `json:"useLocalIp,omitempty"`
	Reciprocal     bool                `json:"reciprocal,omitempty"`
}

type TestHostname struct {
	Target string `json:"target"`
}

type TestIP struct {
	Targets []string `json:"targets"`
}

type TestAgent struct {
	Target string `json:"target"`
}

type TestFlow struct {
	Type                    string `json:"type"`
	Target                  string `json:"target"`
	TargetRefreshIntervalMs int    `json:"targetRefreshIntervalMillis"`
	MaxTasks                int    `json:"maxTasks"`
	InetDirection           string `json:"inetDirection"`
	Direction               string `json:"direction"`
}

type TestSite struct {
	Target string `json:"target"`
}

type TestTag struct {
	Target string `json:"target"`
}

type TestNetworkGrid struct {
	Targets []string `json:"targets"`
}

type TestDNSGrid struct {
	Targets []string `json:"targets"`
	Type    string   `json:"type"`
}

type PingTest struct {
	Timeout  int    `json:"timeout"`
	Count    int    `json:"count"`
	Delay    int    `json:"delay"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

type TraceTest struct {
	Timeout  int    `json:"timeout"`
	Count    int    `json:"count"`
	Delay    int    `json:"delay"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Limit    int    `json:"limit"`
}

type PageLoadTest struct {
	Target          string            `json:"target,omitempty"`
	Timeout         float64           `json:"timeout"`
	Headers         map[string]string `json:"headers,omitempty"`
	CSSSelectors    map[string]string `json:"cssSelectors,omitempty"`
	IgnoreTLSErrors bool              `json:"ignoreTlsErrors,omitempty"`
}

type URLTest struct {
	Target          string            `json:"target,omitempty"`
	Timeout         float64           `json:"timeout"`
	Method          string            `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	CSSSelectors    map[string]string `json:"cssSelectors,omitempty"`
	IgnoreTLSErrors bool              `json:"ignoreTlsErrors,omitempty"`
	Expiry          int               `json:"expiry,omitempty"`
}

type DNSTest struct {
	Target  string   `json:"target,omitempty"`
	Timeout float64  `json:"timeout"`
	Type    string   `json:"type,omitempty"`
	Servers []string `json:"servers,omitempty"`
	Port    int      `json:"port,omitempty"`
}

type TestHealthSettings struct {
	LatencyCritical           float64 `json:"latencyCritical,omitempty"`
	LatencyWarning            float64 `json:"latencyWarning,omitempty"`
	PacketLossCritical        float64 `json:"packetLossCritical,omitempty"`
	PacketLossWarning         float64 `json:"packetLossWarning,omitempty"`
	JitterCritical            float64 `json:"jitterCritical,omitempty"`
	JitterWarning             float64 `json:"jitterWarning,omitempty"`
	HTTPLatencyCritical       float64 `json:"httpLatencyCritical,omitempty"`
	HTTPLatencyWarning        float64 `json:"httpLatencyWarning,omitempty"`
	HTTPValidCodes            []int   `json:"httpValidCodes,omitempty"`
	DNSValidCodes             []int   `json:"dnsValidCodes,omitempty"`
	LatencyCriticalStdDev     float64 `json:"latencyCriticalStddev,omitempty"`
	LatencyWarningStdDev      float64 `json:"latencyWarningStddev,omitempty"`
	JitterCriticalStdDev      float64 `json:"jitterCriticalStddev,omitempty"`
	JitterWarningStdDev       float64 `json:"jitterWarningStddev,omitempty"`
	UnhealthySubtestThreshold float64 `json:"unhealthySubtestThreshold,omitempty"`
	HTTPLatencyCriticalStdDev float64 `json:"httpLatencyCriticalStddev,omitempty"`
	HTTPLatencyWarningStdDev  float64 `json:"httpWarningCriticalStddev,omitempty"`
}

func (c *Client) Tests(ctx context.Context) ([]*Test, error) {
	resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodGet, "/tests", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	testsResponse := &testsResponse{}
	if err := json.Unmarshal(data, &testsResponse); err != nil {
		return nil, err
	}

	return testsResponse.Tests, nil
}

func (c *Client) CreateTest(ctx context.Context, t *Test) (*Test, error) {
	// TODO: perhaps use a default and then merge with param?
	if t.Status == "" {
		t.Status = TestStatusActive
	}
	if t.Settings == nil {
		t.Settings = &TestSettings{}
	}
	if t.Settings.Period == 0 {
		t.Settings.Period = 60
	}
	if t.Settings.HealthSettings == nil {
		t.Settings.HealthSettings = &TestHealthSettings{
			UnhealthySubtestThreshold: 5.0,
		}
	}
	if t.Settings.Family == "" {
		t.Settings.Family = TestIPFamilyV4
	}

	// serialize
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(testCreateRequest{
		Test: t,
	}); err != nil {
		return nil, err
	}

	// DEBUG
	//fmt.Printf("%s\n", string(buf.Bytes()))

	resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodPost, "/tests", buf)
	if err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return nil, errors.New(string(errData))
	}
	defer resp.Body.Close()

	if resp.StatusCode > http.StatusOK {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		return nil, errors.New(string(errData))
	}

	var createResp testCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, err
	}

	return createResp.Test, nil
}

func (c *Client) UpdateTest(ctx context.Context, t *Test, updateFields []string) error {
	mask := []string{}
	for _, field := range updateFields {
		mask = append(mask, "test."+field)
	}
	// ensure required fields are set for api
	if t.Settings.Family == "" {
		t.Settings.Family = TestIPFamilyV4
	}
	// serialize
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(&testUpdateRequest{
		Test: t,
		Mask: strings.Join(mask, ","),
	}); err != nil {
		return err
	}

	resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodPatch, fmt.Sprintf("/tests/%s", t.ID), buf)
	if err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return errors.New(string(errData))
	}

	if resp.StatusCode > http.StatusOK {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return errors.New(string(errData))
	}

	return nil
}

func (c *Client) DeleteTest(ctx context.Context, testID string) error {
	if resp, err := c.request(ctx, syntheticsAPIEndpoint, http.MethodDelete, fmt.Sprintf("/tests/%s", testID), nil); err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(errData))
	}

	return nil
}
