package main

import (
	"fmt"
	"strings"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getUpdateCommand() cli.Command {
	return cli.Command{
		Name:  "update",
		Usage: "update a container",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "set",
				Usage: "specify a variable in the VARIABLE=VALUE form",
			},
			cli.StringFlag{
				Name:  "rebase",
				Usage: "specify a different image",
			},
		},
		Action: func(c *cli.Context) error {
			return updateContainer(c)
		},
	}
}

func updateContainer(c *cli.Context) error {
	set := make(map[string]string)

	for _, s := range c.StringSlice("set") {
		k := strings.SplitN(s, "=", 2)
		if len(k) != 2 {
			return fmt.Errorf("invalid argument %s", s)
		}
		set[k[0]] = k[1]
	}
	rebase := c.String("rebase")

	name := c.Args().First()
	ctx := readContext(c)
	return oc.UpdateContainer(name, set, rebase, ctx)
}
