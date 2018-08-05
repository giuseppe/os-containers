package main

import (
	"log"
	"os"

	oc "github.com/giuseppe/os-containers/pkg/os-containers"
	"github.com/urfave/cli"
)

func readContext(c *cli.Context) *oc.Context {
	ctx := &oc.Context{
		Runtime: c.Parent().String("runtime"),
	}
	return ctx
}

func main() {
	app := cli.NewApp()
	app.Name = "os-container"
	app.Usage = "install and manage system containers"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "runtime",
			Usage: "specify the runtime to use",
		},
	}
	app.Commands = []cli.Command{
		getContainersCommand(),
		getImagesCommand(),
		getPullCommand(),
		getInstallCommand(),
		getUninstallCommand(),
		getUpdateCommand(),
		getRollbackCommand(),
		getRunCommand(),
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
