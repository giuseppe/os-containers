package main

import (
	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getUninstallCommand() cli.Command {
	return cli.Command{
		Name:  "uninstall",
		Usage: "uninstall a container",
		Action: func(c *cli.Context) error {
			return uninstallContainer(c)
		},
	}
}

func uninstallContainer(c *cli.Context) error {
	name := c.Args().First()
	return oc.UninstallContainer(name)
}
