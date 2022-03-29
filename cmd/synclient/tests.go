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
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	cli "github.com/urfave/cli/v2"
)

var testsCommand = &cli.Command{
	Name:  "tests",
	Usage: "manage tests",
	Subcommands: []*cli.Command{
		testsListCommand,
	},
}

var testsListCommand = &cli.Command{
	Name:  "list",
	Usage: "list tests",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "json",
			Usage: "display tests output as json",
		},
	},
	Action: func(clix *cli.Context) error {
		client := getClient(clix)
		ctx := context.Background()
		tests, err := client.Tests(ctx)
		if err != nil {
			return err
		}

		if clix.Bool("json") {
			if err := json.NewEncoder(os.Stdout).Encode(tests); err != nil {
				return err
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 8, 1, 3, ' ', 0)
		fmt.Fprintf(w, "ID\tNAME\tTYPE\n")
		for _, test := range tests {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				test.ID,
				test.Name,
				test.Type,
			)
		}
		w.Flush()
		return nil
	},
}
