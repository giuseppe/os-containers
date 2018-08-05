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
					cli.BoolFlag{
						Name:  "no-truncate",
						Usage: "show full image ID",
					},
				},
				Action: func(c *cli.Context) error {
					return listImages(c.Bool("all"), c.Bool("no-truncate"))
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
				Name:  "tag",
				Usage: "tag an image",
				Action: func(c *cli.Context) error {
					return tagImage(c)
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

func truncateString(s string, l int) string {
	if len(s) > l {
		return s[:l]
	}
	return s
}

func listImages(all bool, noTruncate bool) error {
	images, err := oc.GetImages(all)
	if err != nil {
		return err
	}
	fmtString := "%-42 s%-14s %-20s\n"
	if noTruncate {
		fmtString = "%-42s %-65s %-20s\n"
	}
	fmt.Printf(fmtString, "NAME", "VERSION", "SIZE")
	for _, i := range images {
		name := i.Name
		id := i.ImageID
		if !noTruncate {
			name = truncateString(name, 40)
			id = truncateString(id, 12)
		}
		if name == "" {
			name = "<none>"
		}
		fmt.Printf(fmtString, name, id, fmt.Sprintf("%d", i.Size))

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

func tagImage(c *cli.Context) error {
	src := c.Args().Get(0)
	dest := c.Args().Get(1)
	return oc.TagImage(src, dest)
}
