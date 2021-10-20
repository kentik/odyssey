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

package v1

type Fetch struct {
	// Service is the name of the service for the check
	Service string `json:"service"`
	// Target is the target for the check
	Target string `json:"target"`
	// Method is the http method for the check
	Method string `json:"method"`
	// Period is the interval for which the server to run the check
	Period string `json:"period"`
	// Expiry is the timeout for the check
	Expiry string `json:"expiry"`
}
