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

	"github.com/pkg/errors"
)

const (
	// Test Types
	TestTypeURL = "url"
	// Test Status
	TestStatusActive = "TEST_STATUS_ACTIVE"
	// Test Tasks
	TestTaskHTTP  = "http"
	TestTaskTrace = "trace"
	TestTaskPing  = "ping"
	// Test IP Family
	TestIPFamilyV4   = "IP_FAMILY_V4"
	TestIPFamilyV6   = "IP_FAMILY_V6"
	TestIPFamilyDual = "IP_FAMILY_DUAL"
)

type testCreateRequest struct {
	Test *Test `json:"test"`
}

type testsResponse struct {
	Tests []*Test `json:"tests"`
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
	Period int `json:"period"`
	Count  int `json:"count"`
	Expiry int `json:"expiry"`
	Delay  int `json:"delay"`
}

type TraceTest struct {
	Period   int    `json:"period"`
	Count    int    `json:"count"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Expiry   int    `json:"expiry"`
	Limit    int    `json:"limit"`
	Delay    int    `json:"delay"`
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
	LatencyCritical           int   `json:"latencyCritical"`
	LatencyWarning            int   `json:"latencyWarning"`
	PacketLossCritical        int   `json:"packetLossCritical"`
	PacketLossWarning         int   `json:"packetLossWarning"`
	JitterCritical            int   `json:"jitterCritical"`
	JitterWarning             int   `json:"jitterWarning"`
	HTTPLatencyCritical       int   `json:"httpLatencyCritical"`
	HTTPLatencyWarning        int   `json:"httpLatencyWarning"`
	HTTPValidCodes            []int `json:"httpValidCodes"`
	DNSValidCodes             []int `json:"dnsValidCodes"`
	LatencyCriticalStdDev     int   `json:"latencyCriticalStddev"`
	LatencyWarningStdDev      int   `json:"latencyWarningStddev"`
	JitterCriticalStdDev      int   `json:"jitterCriticalStddev"`
	JitterWarningStdDev       int   `json:"jitterWarningStddev"`
	HTTPLatencyCriticalStdDev int   `json:"httpLatencyCriticalStddev"`
	HTTPLatencyWarningStdDev  int   `json:"httpWarningCriticalStddev"`
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

func (c *Client) CreateTest(ctx context.Context, t *Test) error {
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
	if t.Settings.Family == "" {
		t.Settings.Family = TestIPFamilyV4
	}

	// serialize
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(testCreateRequest{
		Test: t,
	}); err != nil {
		return err
	}

	fmt.Println(string(buf.Bytes()))

	resp, err := c.request(ctx, http.MethodPost, "/tests", buf)
	if err != nil {
		errData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.Wrap(err, string(errData))
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(data))

	return nil
}
