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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type siteResponse struct {
	Sites []*Site `json:"sites,omitempty"`
}

type Site struct {
	ID        int    `json:"id,omit_empty"`
	Name      string `json:"site_name,omit_empty"`
	CompanyID int    `json:"company_id,omit_empty"`
}

func (c *Client) GetSite(ctx context.Context, name string) (*Site, error) {
	resp, err := c.request(ctx, defaultAPIEndpoint, http.MethodGet, "/sites", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	siteResponse := &siteResponse{}
	if err := json.Unmarshal(data, &siteResponse); err != nil {
		return nil, err
	}

	for _, site := range siteResponse.Sites {
		if strings.ToLower(site.Name) == strings.ToLower(name) {
			return site, nil
		}
	}

	return nil, fmt.Errorf("site %s not found", name)
}
