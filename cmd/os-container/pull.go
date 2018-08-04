package main

import (
	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getPullCommand() cli.Command {
	return cli.Command{
		Name:  "pull",
		Usage: "list containers",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "all",
				Usage: "show all containers",
			},
			cli.BoolFlag{
				Name:  "insecure",
				Usage: "allow to pull from an insecure registry",
			},
		},
		Action: func(c *cli.Context) error {
			return pullImage(c)
		},
	}
}

func pullImage(c *cli.Context) error {
	insecure := c.Bool("insecure")
	return oc.PullImage(insecure, c.Args().First())
}
