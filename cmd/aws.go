package cmd

import (
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations/aws"
	"github.com/urfave/cli"
	"log"
)

func buildEC2Cmd(cfg *core.Configuration, manifest *core.Manifest) cli.Command {
	return cli.Command{
		Name: "ec2",
		Usage: "EC2 related related actions. The commands 'bash', 'exec', " +
			"'jstack' and 'jmap' will be executed inside the container.",
		ArgsUsage: "[INSTANCE_NAME] [COMMAND ...]",
		Aliases:   []string{"e"},
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "jump", Usage: "Use the environment jump host."},
			cli.BoolFlag{Name: "all", Usage: "Execute the command on all the instance matched."},
			cli.BoolFlag{Name: "output", Usage: "Saves the stdout of the command to a file."},
		},
		Action: func(c *cli.Context) error {
			var (
				name string
				args []string
			)

			if c.NArg() > 0 {
				name = c.Args().Get(0)
			} else if manifest.Name != "" {
				log.Printf("Manifest found. Using '%v'", name)
				name = manifest.Name
			}

			if c.NArg() > 1 {
				args = c.Args()[1:]
			}

			aws.ConnectToInstance(aws.ConnectionParams{
				Configuration: cfg,
				Filter:        name,
				Output:        c.Bool("output"),
				All:           c.Bool("all"),
				UseJumpHost:   c.Bool("jump"),
				Args:          args},
			)
			return nil
		},
	}
}

func buildRDSCmd(cfg *core.Configuration) cli.Command {
	return cli.Command{

		Name:    "rds",
		Usage:   "RDS actions.",
		Aliases: []string{"r"},
		Action: func(c *cli.Context) error {
			aws.GetRDS(cfg).ConnectToRDSInstance(c.Args().First(), c.Args().Tail())
			return nil
		},
	}
}
