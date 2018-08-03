package main

import (
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
	}
}

func runCommand(c *cli.Context) error {
	container := c.Args().First()
	cmd := []string(c.Args())[1:]
	return oc.RunCommand(container, cmd)
}
