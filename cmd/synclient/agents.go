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
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

var agentsCommand = &cli.Command{
	Name:  "agents",
	Usage: "manage agents",
	Subcommands: []*cli.Command{
		agentsListCommand,
		agentsAuthorizeCommand,
		agentsDeleteCommand,
	},
}

var agentsListCommand = &cli.Command{
	Name:  "list",
	Usage: "list agents",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "all",
			Usage: "show all agents",
			Value: false,
		},
	},
	Action: func(clix *cli.Context) error {
		client := getClient(clix)
		ctx := context.Background()
		agents, err := client.Agents(ctx)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 8, 1, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tID\tTYPE\tSTATUS\tLOCATION\tOS\tIP\tSITE\tVERSION\n")
		for _, agent := range agents {
			if !clix.Bool("all") && agent.Type != "private" {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				agent.Alias,
				agent.ID,
				agent.Type,
				agent.Status,
				agent.Name,
				agent.OS,
				agent.IP,
				agent.SiteID,
				agent.Version,
			)
		}
		w.Flush()
		return nil
	},
}

var agentsAuthorizeCommand = &cli.Command{
	Name:  "authorize",
	Usage: "authorize agent",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "id",
			Usage:   "agent id",
			Aliases: []string{"i"},
		},
		&cli.StringFlag{
			Name:    "name",
			Usage:   "agent name",
			Aliases: []string{"n"},
		},
		&cli.StringFlag{
			Name:    "site-id",
			Aliases: []string{"s"},
			Usage:   "site ID",
		},
	},
	Action: func(clix *cli.Context) error {
		client := getClient(clix)
		ctx := context.Background()

		id := clix.String("id")
		if id == "" {
			return fmt.Errorf("id must be specified")
		}

		name := clix.String("name")
		if id == "" {
			return fmt.Errorf("name must be specified")
		}

		siteID := clix.String("site-id")
		if siteID == "" {
			return fmt.Errorf("site-id must be specified")
		}

		siteName := clix.String("site-name")
		if siteName == "" {
			return fmt.Errorf("site-name must be specified")
		}

		if err := client.AuthorizeAgent(ctx, id, name, siteID); err != nil {
			return err
		}
		logrus.Infof("authorized %s", id)

		return nil
	},
}

var agentsDeleteCommand = &cli.Command{
	Name:  "delete",
	Usage: "delete agent",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "id",
			Usage:   "agent id",
			Aliases: []string{"i"},
			Value:   &cli.StringSlice{},
		},
	},
	Action: func(clix *cli.Context) error {
		client := getClient(clix)
		ctx := context.Background()

		for _, id := range clix.StringSlice("id") {
			if err := client.DeleteAgent(ctx, id); err != nil {
				return err
			}
			logrus.Infof("deleted agent %s", id)
		}

		return nil
	},
}
