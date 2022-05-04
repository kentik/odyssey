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
)

type userResponse struct {
	Users []*User `json:"users,omitempty"`
}

type User struct {
	ID        string `json:"id,omit_empty"`
	Username  string `json:"username,omit_empty"`
	FullName  string `json:"user_full_name,omit_empty"`
	Email     string `json:"user_email,omitempty"`
	CompanyID string `json:"company_id,omitempty"`
}

func (c *Client) GetUser(ctx context.Context, email string) (*User, error) {
	resp, err := c.request(ctx, defaultAPIEndpoint, http.MethodGet, "/users", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	userResponse := &userResponse{}
	if err := json.Unmarshal(data, &userResponse); err != nil {
		return nil, err
	}

	for _, u := range userResponse.Users {
		if u.Email == email {
			return u, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}
