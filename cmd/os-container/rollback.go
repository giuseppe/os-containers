package main

import (
	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getRollbackCommand() cli.Command {
	return cli.Command{
		Name:  "rollback",
		Usage: "rollback a container",
		Action: func(c *cli.Context) error {
			return rollbackContainer(c)
		},
	}
}

func rollbackContainer(c *cli.Context) error {
	name := c.Args().First()
	return oc.RollbackContainer(name)
}
