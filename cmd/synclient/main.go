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

package main

import (
	"fmt"
	"os"

	"github.com/kentik/odyssey/pkg/synthetics"
	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	app := &cli.App{
		Name:  "synclient",
		Usage: "Kentik Synthetics API Client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "kentik-email",
				Usage:   "Kentik Email",
				Aliases: []string{"e"},
				EnvVars: []string{"KENTIK_EMAIL"},
			},
			&cli.StringFlag{
				Name:    "kentik-api-token",
				Usage:   "Kentik API Token",
				Aliases: []string{"t"},
				EnvVars: []string{"KENTIK_API_TOKEN"},
			},
		},
		Before: func(clix *cli.Context) error {
			if clix.String("kentik-email") == "" || clix.String("kentik-api-token") == "" {
				cli.ShowAppHelp(clix)
				return fmt.Errorf("kentik-email and kentik-api-token must be specified")
			}
			return nil
		},
		Commands: []*cli.Command{
			agentsCommand,
			testsCommand,
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func getClient(clix *cli.Context) *synthetics.Client {
	logger := zap.New()
	return synthetics.NewClient(clix.String("kentik-email"), clix.String("kentik-api-token"), logger)
}
