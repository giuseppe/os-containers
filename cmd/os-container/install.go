package main

import (
	"fmt"
	"strings"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getInstallCommand() cli.Command {
	return cli.Command{
		Name:  "install",
		Usage: "install a container",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "set",
				Usage: "specify a variable in the VARIABLE=VALUE form",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "specify the name for the container",
			},
		},
		Action: func(c *cli.Context) error {
			return installContainer(c)
		},
	}
}

func installContainer(c *cli.Context) error {
	set := make(map[string]string)

	for _, s := range c.StringSlice("set") {
		k := strings.SplitN(s, "=", 2)
		if len(k) != 2 {
			return fmt.Errorf("invalid argument %s", s)
		}
		set[k[0]] = k[1]
	}
	name := c.String("name")
	image := c.Args().First()
	ctx := readContext(c)
	return oc.InstallContainer(name, image, set, ctx)
}
