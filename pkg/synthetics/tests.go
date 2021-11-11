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
	DeviceID string        `json:"deviceId"`
	Status   string        `json:"status"`
	Settings *TestSettings `json:"settings"`
}

type TestSettings struct {
	Hostname           *TestHostname           `json:"hostname"`
	IP                 *TestIP                 `json:"ip"`
	Agent              *TestAgent              `json:"agent"`
	Flow               *TestFlow               `json:"flow"`
	Site               *TestSite               `json:"site"`
	Tag                *TestTag                `json:"tag"`
	DNS                *DNSTest                `json:"dns"`
	URL                *URLTest                `json:"url"`
	NetworkGrid        *TestNetworkGrid        `json:"networkGrid"`
	PageLoad           *PageLoadTest           `json:"pageLoad"`
	DNSGrid            *TestDNSGrid            `json:"dnsGrid"`
	AgentIDs           []string                `json:"agentIds"`
	Period             int                     `json:"period"`
	Count              int                     `json:"count"`
	Expiry             int                     `json:"expiry"`
	Limit              int                     `json:"limit"`
	Tasks              []string                `json:"tasks"`
	HealthSettings     *TestHealthSettings     `json:"healthSettings"`
	MonitoringSettings *TestMonitoringSettings `json:"monitoringSettings"`
	Ping               *PingTest               `json:"ping"`
	Trace              *TraceTest              `json:"trace"`
	Port               int                     `json:"port"`
	Protocol           string                  `json:"protocol"`
	Family             string                  `json:"family"`
	Servers            []string                `json:"servers"`
	UseLocalIP         bool                    `json:"useLocalIp"`
	Reciprocal         bool                    `json:"reciprocal"`
	RollupLevel        int                     `json:"rollup_level"`
	HTTP               *TestHTTP               `json:"http"`
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
	Target                  string `json:"target"`
	TargetRefreshIntervalMs int    `json:"targetRefreshIntervalMillis"`
	MaxTasks                int    `json:"maxTasks"`
	Type                    string `json:"type"`
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
	Period float64 `json:"period"`
	Count  float64 `json:"count"`
	Expiry float64 `json:"expiry"`
	Delay  float64 `json:"delay"`
}

type TraceTest struct {
	Period   float64 `json:"period"`
	Count    float64 `json:"count"`
	Protocol string  `json:"protocol"`
	Port     int     `json:"port"`
	Expiry   float64 `json:"expiry"`
	Limit    float64 `json:"limit"`
	Delay    float64 `json:"delay"`
}

type PageLoadTest struct {
	Target string `json:"target"`
}

type URLTest struct {
	Target string `json:"target"`
}

type DNSTest struct {
	Target string `json:"target"`
	Type   string `json:"type"`
}

type TestHealthSettings struct {
	LatencyCritical           float64 `json:"latencyCritical"`
	LatencyWarning            float64 `json:"latencyWarning"`
	PacketLossCritical        float64 `json:"packetLossCritical"`
	PacketLossWarning         float64 `json:"packetLossWarning"`
	JitterCritical            float64 `json:"jitterCritical"`
	JitterWarning             float64 `json:"jitterWarning"`
	HTTPLatencyCritical       float64 `json:"httpLatencyCritical"`
	HTTPLatencyWarning        float64 `json:"httpLatencyWarning"`
	HTTPValidCodes            []int   `json:"httpValidCodes"`
	DNSValidCodes             []int   `json:"dnsValidCodes"`
	LatencyCriticalStdDev     float64 `json:"latencyCriticalStddev"`
	LatencyWarningStdDev      float64 `json:"latencyWarningStddev"`
	JitterCriticalStdDev      float64 `json:"jitterCriticalStddev"`
	JitterWarningStdDev       float64 `json:"jitterWarningStddev"`
	HTTPLatencyCriticalStdDev float64 `json:"httpLatencyCriticalStddev"`
	HTTPLatencyWarningStdDev  float64 `json:"httpWarningCriticalStddev"`
}

type TestMonitoringSettings struct {
	ActivationGracePeriod string   `json:"activationGracePeriod"`
	ActivationTimeUnit    string   `json:"activationTimeUnit"`
	ActivationTimeWindow  string   `json:"activationTimeWindow"`
	ActivationTimes       string   `json:"activationTimes"`
	NotificationChannels  []string `json:"notificationChannels"`
}

type TestHTTP struct {
	Period          int               `json:"period"`
	Expiry          int               `json:"expiry"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IgnoreTLSErrors bool              `json:"ignoreTlsErrors"`
	CSSSelectors    map[string]string `json:"cssSelectors"`
}

func (c *Client) Tests(ctx context.Context) ([]*Test, error) {
	resp, err := c.request(ctx, http.MethodGet, "/tests", nil)
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
	// validation
	if t.DeviceID == "" {
		t.DeviceID = "0"
	}
	if t.Status == "" {
		t.Status = TestStatusActive
	}
	if t.Settings == nil {
		t.Settings = &TestSettings{}
	}
	if t.Settings.Period == 0 {
		t.Settings.Period = 60
	}
	if t.Settings.Expiry == 0 {
		t.Settings.Expiry = 5000
	}
	if t.Settings.HealthSettings == nil {
		t.Settings.HealthSettings = &TestHealthSettings{}
	}
	if t.Settings.MonitoringSettings == nil {
		t.Settings.MonitoringSettings = &TestMonitoringSettings{
			ActivationTimeUnit:   "seconds",
			ActivationTimeWindow: "30",
			ActivationTimes:      "3",
		}
	}
	if t.Settings.Ping == nil {
		t.Settings.Ping = &PingTest{}
	}
	if t.Settings.Trace == nil {
		t.Settings.Trace = &TraceTest{}
	}
	if t.Settings.Family == "" {
		t.Settings.Family = TestIPFamilyV4
	}
	// set minimum defaults for Kentik API
	if t.Settings.Ping != nil && t.Settings.Ping.Period < pingPeriodMinimum {
		t.Settings.Ping.Period = pingPeriodMinimum
	}
	// set minimum defaults for Kentik API
	if t.Settings.Trace != nil && t.Settings.Trace.Period < tracePeriodMinimum {
		t.Settings.Trace.Period = tracePeriodMinimum
	}

	// serialize
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(testCreateRequest{
		Test: t,
	}); err != nil {
		return nil, err
	}

	resp, err := c.request(ctx, http.MethodPost, "/tests", buf)
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

	resp, err := c.request(ctx, http.MethodPatch, fmt.Sprintf("/tests/%s", t.ID), buf)
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
	if resp, err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/tests/%s", testID), nil); err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(errData))
	}

	return nil
}
