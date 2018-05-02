package cmd

import (
	"github.com/j-martin/bub/core"
	"github.com/j-martin/bub/integrations"
	"github.com/j-martin/bub/integrations/atlassian"
	"github.com/j-martin/bub/integrations/aws"
	"github.com/j-martin/bub/integrations/ci"
	"github.com/j-martin/bub/integrations/github"
	"github.com/j-martin/bub/integrations/vault"
	"github.com/j-martin/bub/utils"
	"github.com/urfave/cli"
	"log"
	"os"
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
			vault.MustSetupVault(cfg)
			log.Println("Done.")
			return nil
		},
	}
}

func buildCircleCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "trigger",
			Usage:   "Trigger the current branch of the current repo and wait for success.",
			Aliases: []string{"t"},
			Action: func(c *cli.Context) error {
				return ci.MustInitCircle(cfg).TriggerAndWaitForSuccess(manifest)
			},
		},
		{
			Name:    "check",
			Usage:   "Check the build status of the current commit.",
			Aliases: []string{"c"},
			Action: func(c *cli.Context) error {
				return ci.MustInitCircle(cfg).CheckBuildStatus(manifest)
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
					err := core.InitGit().FetchTags()
					if err != nil {
						return err
					}
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
