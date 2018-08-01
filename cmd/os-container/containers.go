package main

import (
	"fmt"
	"time"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getContainersCommand() cli.Command {
	return cli.Command{
		Name:  "containers",
		Usage: "manage containers",
		Subcommands: []cli.Command{
			{
				Name:  "list",
				Usage: "list containers",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "all",
						Usage: "show all containers",
					},
				},
				Action: func(c *cli.Context) error {
					return listContainers(c.Bool("all"))
				},
			},
		},
	}
}

func getCreated(c int64) string {
	t := time.Unix(c, 0)

	return fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
}

func listContainers(all bool) error {
	containers, err := oc.GetContainers(all)
	if err != nil {
		return err
	}
	fmtString := "%-10s %-20s %-20s %-10s %-15s\n"
	fmt.Printf(fmtString, "NAME", "IMAGE", "CREATED", "STATE", "RUNTIME")
	for _, c := range containers {
		status, err := c.ContainerStatus()
		if err != nil {
			return err
		}
		if !all && status != oc.Running {
			continue
		}

		statusString := oc.GetContainerStatusString(status)
		fmt.Printf(fmtString, c.Name, c.Image, getCreated(c.Created), statusString, c.Runtime)

	}
	return nil
}
