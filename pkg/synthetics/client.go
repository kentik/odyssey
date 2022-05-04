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
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"
)

const (
	syntheticsAPIEndpoint = "https://grpc.api.kentik.com/synthetics/v202202"
	defaultAPIEndpoint    = "https://api.kentik.com/api/v5"
	headerAuthEmail       = "x-ch-auth-email"
	headerAuthToken       = "x-ch-auth-api-token"
)

type Client struct {
	email string
	token string
	log   logr.Logger
}

// NewClient returns a new Sythentics API client
func NewClient(email, token string, log logr.Logger) *Client {
	return &Client{
		email: email,
		token: token,
		log:   log,
	}
}

func (c *Client) request(ctx context.Context, endpoint, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", endpoint, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add(headerAuthEmail, c.email)
	req.Header.Add(headerAuthToken, c.token)
	httpClient := &http.Client{}

	return httpClient.Do(req)
}
