package main

import (
	"fmt"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func getImagesCommand() cli.Command {
	return cli.Command{
		Name:  "images",
		Usage: "manage images",
		Subcommands: []cli.Command{
			{
				Name:  "list",
				Usage: "list images",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "all",
						Usage: "show all images",
					},
				},
				Action: func(c *cli.Context) error {
					return listImages(c.Bool("all"))
				},
			},
			{
				Name:  "delete",
				Usage: "delete an image",
				Action: func(c *cli.Context) error {
					return deleteImage(c)
				},
			},
			{
				Name:  "prune",
				Usage: "prune unused images",
				Action: func(c *cli.Context) error {
					return pruneImages()
				},
			},
		},
	}
}

func listImages(all bool) error {
	images, err := oc.GetImages(all)
	if err != nil {
		return err
	}
	fmtString := "%-50s\n"
	fmt.Printf(fmtString, "NAME")
	for _, i := range images {
		fmt.Printf(fmtString, i.Name)

	}
	return nil
}

func deleteImage(c *cli.Context) error {
	image := c.Args().First()
	return oc.DeleteImage(image)
}

func pruneImages() error {
	return oc.PruneImages()
}
