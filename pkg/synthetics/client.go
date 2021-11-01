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
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultAPIEndpoint = "https://synthetics.api.kentik.com/synthetics/v202101beta1"
	headerAuthEmail    = "x-ch-auth-email"
	headerAuthToken    = "x-ch-auth-api-token"
)

type Client struct {
	endpoint string
	email    string
	token    string
}

// NewClient returns a new Sythentics API client
func NewClient(email, token string) *Client {
	return &Client{
		endpoint: defaultAPIEndpoint,
		email:    email,
		token:    token,
	}
}

func (c *Client) request(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", defaultAPIEndpoint, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add(headerAuthEmail, c.email)
	req.Header.Add(headerAuthToken, c.token)
	httpClient := &http.Client{}

	return httpClient.Do(req)
}
