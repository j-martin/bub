package cmd

import (
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/integrations/aws"
	"github.com/benchlabs/bub/integrations/ci"
	"github.com/benchlabs/bub/integrations/github"
	"github.com/benchlabs/bub/utils"
	"github.com/urfave/cli"
	"log"
	"os"
	"strings"
)

func buildSetupCmd() cli.Command {
	resetCredentials := "reset-credentials"
	return cli.Command{
		Name:  "setup",
		Usage: "Setup bub on your machine.",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: resetCredentials, Usage: "Prompt you to re-enter credentials."},
		},
		Action: func(c *cli.Context) error {
			core.MustSetupConfig()
			// Reloading the config
			cfg, _ := core.LoadConfiguration()
			cfg.ResetCredentials = c.Bool(resetCredentials)
			aws.MustSetupConfig()
			atlassian.MustSetupJIRA(cfg)
			atlassian.MustSetupConfluence(cfg)
			github.MustSetupGitHub(cfg)
			ci.MustSetupJenkins(cfg)
			log.Println("Done.")
			return nil
		},
	}
}

func buildConfigCmd() cli.Command {
	showDefaults := "show-default"
	shared := "shared"
	return cli.Command{
		Name:  "config",
		Usage: "Edit your bub config",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: showDefaults, Usage: "Show default config for reference"},
			cli.BoolFlag{Name: shared, Usage: "Edit shared config."},
		},
		Action: func(c *cli.Context) error {
			if c.Bool(showDefaults) {
				print(core.GetConfigString())
			} else if c.Bool(shared) {
				core.EditConfiguration(core.ConfigSharedFile)
			} else {
				core.EditConfiguration(core.ConfigUserFile)
			}
			return nil
		},
	}
}

func getRegion(environment string, cfg *core.Configuration, c *cli.Context) string {
	region := c.String("region")
	if region == "" {
		prefix := strings.Split(environment, "-")[0]
		for _, i := range cfg.AWS.Environments {
			if i.Prefix == prefix {
				return i.Region
			}
		}
		return cfg.AWS.Regions[0]
	}
	return region
}

func buildCircleCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "trigger",
			Usage:   "Trigger the current branch of the current repo and wait for success.",
			Aliases: []string{"t"},
			Action: func(c *cli.Context) error {
				ci.TriggerAndWaitForSuccess(cfg, manifest)
				return nil
			},
		},
		{
			Name:    "open",
			Usage:   "Open Circle for the current repository.",
			Aliases: []string{"t"},
			Action: func(c *cli.Context) error {
				return ci.OpenCircle(cfg, manifest, false)
			},
		},
		{
			Name:    "circle",
			Usage:   "Opens the result for the current branch.",
			Aliases: []string{"b"},
			Action: func(c *cli.Context) error {
				return ci.OpenCircle(cfg, manifest, true)
			},
		},
	}
}

func buildSplunkCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "production",
			Aliases: []string{"p"},
			Usage:   "Open the service production logs.",
			Action: func(c *cli.Context) error {
				return integrations.OpenSplunk(cfg, manifest, false)
			},
		},
		{
			Name:    "staging",
			Aliases: []string{"s"},
			Usage:   "Open the service staging logs.",
			Action: func(c *cli.Context) error {
				return integrations.OpenSplunk(cfg, manifest, true)
			},
		},
	}
}

func buildRepositoryCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	slackFormat := "slack-format"
	noSlackAt := "slack-no-at"
	noFetch := "no-fetch"
	return []cli.Command{
		{
			Name:  "synchronize",
			Usage: "Synchronize the all the active repositories.",
			Action: func(c *cli.Context) error {
				message := `

STOP!

This command will clone and/or Update all 'active' Bench repositories.
Existing work will be stashed and pull the master branch. Your work won't be lost, but be careful.
Please make sure you are in the directory where you store your repos and not a specific repo.

Continue?`
				if !c.Bool("force") && !utils.AskForConfirmation(message) {
					os.Exit(1)
				}
				return core.SyncRepositories()
			},
		},
		{
			Name:    "pending",
			Aliases: []string{"p"},
			Usage:   "List diff between the previous version and the next one.",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: slackFormat, Usage: "Format the result for slack."},
				cli.BoolFlag{Name: noSlackAt, Usage: "Do not add @person at the end."},
				cli.BoolFlag{Name: noFetch, Usage: "Do not fetch tags."},
			},
			Action: func(c *cli.Context) error {
				if !c.Bool(noFetch) {
					core.InitGit().FetchTags()
				}
				previousVersion := "production"
				if len(c.Args()) > 0 {
					previousVersion = c.Args().Get(0)
				}
				nextVersion := "HEAD"
				if len(c.Args()) > 1 {
					nextVersion = c.Args().Get(1)
				}
				core.InitGit().PendingChanges(cfg, manifest, previousVersion, nextVersion, c.Bool(slackFormat), c.Bool(noSlackAt))
				return nil
			},
		},
	}
}