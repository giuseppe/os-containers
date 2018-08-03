package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "os-container"
	app.Usage = "install and manage system containers"
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
