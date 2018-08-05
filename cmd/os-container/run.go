package main

import (
	"fmt"
	"strings"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getRunCommand() cli.Command {
	return cli.Command{
		Name:  "run",
		Usage: "run a command in a container",
		Action: func(c *cli.Context) error {
			return runCommand(c)
		},
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "set",
				Usage: "specify a variable in the VARIABLE=VALUE form",
			},
		},
	}
}

func runCommand(c *cli.Context) error {
	set := make(map[string]string)

	for _, s := range c.StringSlice("set") {
		k := strings.SplitN(s, "=", 2)
		if len(k) != 2 {
			return fmt.Errorf("invalid argument %s", s)
		}
		set[k[0]] = k[1]
	}
	container := c.Args().First()
	cmd := []string(c.Args())[1:]
	ctx := readContext(c)
	return oc.RunCommand(container, cmd, set, ctx)
}
