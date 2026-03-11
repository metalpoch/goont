package main

import (
	"context"
	"goont/commands"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name:     "olt",
				Usage:    "opciones para trabajar con los OLT",
				Commands: []*cli.Command{commands.OltList, commands.OltAdd, commands.OltRemove},
			},
			{
				Name:     "ont",
				Usage:    "opciones para trabajar con los ONT",
				Commands: []*cli.Command{commands.OntScan},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
