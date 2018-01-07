package cmd

import (
	"errors"
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/utils"
	"github.com/urfave/cli"
	"os"
)

func buildConfluenceCmds(cfg *core.Configuration) []cli.Command {
	return []cli.Command{
		{
			Name:    "open",
			Usage:   "Open Confluence",
			Aliases: []string{"o"},
			Action: func(c *cli.Context) error {
				return utils.OpenURI(cfg.Confluence.Server)
			},
		},
		{
			Name:    "search",
			Usage:   "CQL",
			Aliases: []string{"s"},
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "noop", Usage: "No Op."},
			},
			Action: func(c *cli.Context) error {
				if len(c.Args()) == 0 {
					return errors.New("not enough args")
				}
				return atlassian.MustInitConfluence(cfg).SearchAndOpen(c.Args()...)
			},
		},
		{
			Name:    "search-and-replace",
			Usage:   "CQL OLD_STRING NEW_STRING",
			Aliases: []string{"r"},
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "noop", Usage: "No Op."},
			},
			Action: func(c *cli.Context) error {
				if len(c.Args()) != 3 {
					return errors.New("not enough args")
				}
				if !utils.AskForConfirmation("This may modify a lot of pages, are you sure?") {
					os.Exit(1)
				}
				return atlassian.MustInitConfluence(cfg).SearchAndReplace(
					c.Args().Get(0),
					c.Args().Get(1),
					c.Args().Get(2),
					c.Bool("noop"),
				)
			},
		},
	}
}
