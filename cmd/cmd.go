package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/integrations/aws"
	"github.com/benchlabs/bub/integrations/ci"
	"github.com/benchlabs/bub/utils"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

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

func Init(app *cli.App, cfg *core.Configuration) {
	manifest, manifestErr := core.LoadManifest("")
	manifestCommands := []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"l"},
			Usage:   "List all manifests.",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "full", Usage: "Display all information, including readmes and changelogs."},
				cli.BoolFlag{Name: "active", Usage: "Display only active projects."},
				cli.BoolFlag{Name: "name", Usage: "Display only the project names."},
				cli.BoolFlag{Name: "service", Usage: "Display only the services projects."},
				cli.BoolFlag{Name: "lib", Usage: "Display only the library projects."},
				cli.StringFlag{Name: "lang", Usage: "Display only projects matching the language"},
			},
			Action: func(c *cli.Context) error {
				manifests := core.GetManifestRepository().GetAllManifests()
				for _, m := range manifests {
					if !c.Bool("full") {
						m.Readme = ""
						m.ChangeLog = ""
					}

					if c.Bool("active") && !m.Active {
						continue
					}

					if c.Bool("service") && !core.IsSameType(m, "service") {
						continue
					}

					if c.Bool("lib") && !core.IsSameType(m, "library") {
						continue
					}

					if c.String("lang") != "" && m.Language != c.String("lang") {
						continue
					}

					if c.Bool("name") {
						fmt.Println(m.Name)
					} else {
						yml, _ := yaml.Marshal(m)
						fmt.Println(string(yml))
					}
				}
				return nil
			},
		},
		{
			Name:    "create",
			Aliases: []string{"c"},
			Usage:   "Creates a base manifest.",
			Action: func(c *cli.Context) error {
				core.CreateManifest()
				return nil
			},
		},
		{
			Name:    "graph",
			Aliases: []string{"g"},
			Usage:   "Creates dependency graph from manifests.",
			Action: func(c *cli.Context) error {
				generateGraphs()
				return nil
			},
		},
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Updates/uploads the manifest to the database.",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "artifact-version"},
			},
			Action: func(c *cli.Context) error {
				if manifestErr != nil {
					log.Fatal(manifestErr)
					os.Exit(1)
				}
				manifest.Version = c.String("artifact-version")
				core.GetManifestRepository().StoreManifest(manifest)
				return atlassian.MustInitConfluence(cfg).UpdateDocumentation(manifest)
			},
		},
		{
			Name:    "validate",
			Aliases: []string{"v"},
			Usage:   "Validates the manifest.",
			Action: func(c *cli.Context) error {
				//TODO: Build proper validation
				if manifestErr != nil {
					log.Fatal(manifestErr)
					os.Exit(1)
				}
				manifest.Version = c.String("artifact-version")
				yml, _ := yaml.Marshal(manifest)
				log.Println(string(yml))
				return nil
			},
		},
	}
	repositoryCommands := []cli.Command{
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
				cli.BoolFlag{Name: "slack-format", Usage: "Format the result for slack."},
				cli.BoolFlag{Name: "slack-no-at", Usage: "Do not add @person at the end."},
				cli.BoolFlag{Name: "no-fetch", Usage: "Do not fetch tags."},
			},
			Action: func(c *cli.Context) error {
				if !c.Bool("no-fetch") {
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
				core.InitGit().PendingChanges(cfg, manifest, previousVersion, nextVersion, c.Bool("slack-format"), c.Bool("slack-no-at"))
				return nil
			},
		},
	}
	ebCommands := []cli.Command{
		{
			Name:    "environments",
			Aliases: []string{"env"},
			Usage:   "List enviroments and their states.",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
			},
			Action: func(c *cli.Context) error {
				aws.ListEnvironments(cfg)
				return nil
			},
		},
		{
			Name:      "events",
			Aliases:   []string{"e"},
			Usage:     "List events for all environments.",
			UsageText: "[ENVIRONMENT_NAME] Optional filter by environment name.",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
				cli.BoolFlag{Name: "reverse"},
			},
			Action: func(c *cli.Context) error {
				environment := ""
				if c.NArg() > 0 {
					environment = c.Args().Get(0)
				} else if manifestErr == nil {
					environment = "pro-" + manifest.Name
					log.Printf("Manifest found. Using '%v'", environment)
				}
				aws.ListEvents(getRegion(environment, cfg, c), environment, time.Time{}, c.Bool("reverse"), true, false)
				return nil
			},
		},
		{
			Name:      "ready",
			Aliases:   []string{"r"},
			Usage:     "Wait for environment to be ready.",
			UsageText: "ENVIRONMENT_NAME",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
			},
			Action: func(c *cli.Context) error {
				environment := ""
				if c.NArg() > 0 {
					environment = c.Args().Get(0)
				} else if manifestErr == nil {
					environment = "pro-" + manifest.Name
					log.Printf("Manifest found. Using '%v'", environment)
				}
				aws.EnvironmentIsReady(getRegion(environment, cfg, c), environment, true)
				return nil
			},
		},
		{
			Name:      "settings",
			Aliases:   []string{"s"},
			Usage:     "List Environment settings",
			UsageText: "ENVIRONMENT_NAME",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
				cli.BoolFlag{Name: "all", Usage: "Display all settings, not just environment variables."},
			},
			Action: func(c *cli.Context) error {
				environment := ""
				if c.NArg() > 0 {
					environment = c.Args().Get(0)
				} else if manifestErr == nil {
					environment = "pro-" + manifest.Name
					log.Printf("Manifest found. Using '%v'", environment)
				}
				aws.DescribeEnvironment(getRegion(environment, cfg, c), environment, c.Bool("all"))
				return nil
			},
		},
		{
			Name:      "versions",
			Aliases:   []string{"v"},
			Usage:     "List all versions available.",
			ArgsUsage: "[APPLICATION_NAME] Optional, limits the versions to the application name.",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
			},
			Action: func(c *cli.Context) error {
				application := ""
				if c.NArg() > 0 {
					application = c.Args().Get(0)
				} else if manifestErr == nil {
					application = manifest.Name
					log.Printf("Manifest found. Using '%v'", application)
				}

				aws.ListApplicationVersions(getRegion(application, cfg, c), application)
				return nil
			},
		},
		{
			Name:      "deploy",
			Aliases:   []string{"d"},
			Usage:     "Deploy version to an environment.",
			ArgsUsage: "[ENVIRONMENT_NAME] [VERSION]",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
			},
			Action: func(c *cli.Context) error {
				environment := ""
				if c.NArg() > 0 {
					environment = c.Args().Get(0)
				} else if manifestErr == nil {
					environment = "pro-" + manifest.Name
					log.Printf("Manifest found. Using '%v'", environment)
				} else {
					log.Fatal("Environment required. Stopping.")
					os.Exit(1)
				}

				region := getRegion(environment, cfg, c)

				if c.NArg() < 2 {
					aws.ListApplicationVersions(region, aws.GetApplication(environment))
					log.Println("Version required. Specify one of the application versions above.")
					os.Exit(2)
				}
				version := c.Args().Get(1)
				aws.DeployVersion(region, environment, version)
				return nil
			},
		},
	}
	gitHubCommands := []cli.Command{
		{
			Name:    "repo",
			Aliases: []string{"r"},
			Usage:   "Open repo in your browser.",
			Action: func(c *cli.Context) error {
				return integrations.MustInitGitHub(cfg).OpenPage(manifest)
			},
		},
		{
			Name:    "issues",
			Aliases: []string{"i"},
			Usage:   "Open issues list in your browser.",
			Action: func(c *cli.Context) error {
				return integrations.MustInitGitHub(cfg).OpenPage(manifest, "issues")
			},
		},
		{
			Name:    "branches",
			Aliases: []string{"b"},
			Usage:   "Open branches list in your browser.",
			Action: func(c *cli.Context) error {
				return integrations.MustInitGitHub(cfg).OpenPage(manifest, "branches")
			},
		},
		{
			Name:    "pr",
			Aliases: []string{"p"},
			Usage:   "Open Pull Request list in your browser.",
			Action: func(c *cli.Context) error {
				return integrations.MustInitGitHub(cfg).OpenPage(manifest, "pulls")
			},
		},
		{
			Name:  "stale-branches",
			Usage: "Open repo in your browser.",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "max-age", Value: "30"},
			},
			Action: func(c *cli.Context) error {
				return integrations.MustInitGitHub(cfg).ListBranches(c.Int("max-age"))
			},
		},
	}
	jenkinsCommands := []cli.Command{
		{
			Name:    "console",
			Aliases: []string{"c"},
			Usage:   "Opens the master build..",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).OpenPage()
			},
		},
		{
			Name:    "console",
			Aliases: []string{"c"},
			Usage:   "Opens the (web) console of the last build of master.",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).OpenPage("lastBuild/consoleFull")
			},
		},
		{
			Name:    "jobs",
			Aliases: []string{"j"},
			Usage:   "Shows the console output of the last build.",
			Action: func(c *cli.Context) error {
				ci.MustInitJenkins(cfg, manifest).ShowConsoleOutput()
				return nil
			},
		},
		{
			Name:    "artifacts",
			Aliases: []string{"a"},
			Usage:   "Get the previous build's artifacts.",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).GetArtifacts()
			},
		},
		{
			Name:    "build",
			Aliases: []string{"b"},
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "no-wait", Usage: "Do not wait for the job to be completed."},
				cli.BoolFlag{Name: "force", Usage: "Trigger job regardless if a build running."},
			},
			Usage: "Trigger build of the current branch.",
			Action: func(c *cli.Context) error {
				ci.MustInitJenkins(cfg, manifest).BuildJob(c.Bool("no-wait"), c.Bool("force"))
				return nil
			},
		},
	}
	jiraSearchIssue := cli.Command{
		Name:    "search",
		Aliases: []string{"s"},
		Usage:   "Search and open JIRA issue in the browser.",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "all", Usage: "Use all projects."},
			cli.BoolFlag{Name: "resolved", Usage: "Include resolved issues."},
			cli.StringFlag{Name: "p", Usage: "Specify the project."},
		},
		Action: func(c *cli.Context) error {
			project := c.String("pr")
			if !c.Bool("all") {
				project = cfg.JIRA.Project
			}
			return atlassian.MustInitJIRA(cfg).SearchIssueText(strings.Join(c.Args(), " "), project, c.Bool("resolved"))
		},
	}

	jiraOpenIssue := cli.Command{
		Name:    "open",
		Aliases: []string{"o"},
		Usage:   "Open JIRA issue in the browser.",
		Action: func(c *cli.Context) error {
			var key string
			if len(c.Args()) > 0 {
				key = c.Args().Get(0)
			}
			return atlassian.MustInitJIRA(cfg).OpenIssue(key)
		},
	}
	jiraClaimIssue := cli.Command{

		Name:    "claim",
		Aliases: []string{"cl"},
		Usage:   "Claim unassigned issue in the current sprint.",
		Action: func(c *cli.Context) error {
			var issueKey string
			if len(c.Args()) > 0 {
				issueKey = c.Args().Get(0)
			}
			return atlassian.MustInitJIRA(cfg).ClaimIssueInActiveSprint(issueKey)
		},
	}
	jiraTransitionIssue := cli.Command{
		Name:    "transition",
		Aliases: []string{"t"},
		Usage:   "Transition issue based on current branch.",
		Action: func(c *cli.Context) error {
			var transition string
			if len(c.Args()) == 0 {
				transition = c.Args().Get(0)
			}
			return atlassian.MustInitJIRA(cfg).TransitionIssue("", transition)
		},
	}

	jiraOpenBoard := cli.Command{
		Name:    "board",
		Aliases: []string{"b"},
		Usage:   "Open your JIRA board.",
		Action: func(c *cli.Context) error {
			return utils.OpenURI(cfg.JIRA.Server, "secure/RapidBoard.jspa?rapidView="+cfg.JIRA.Board)
		},
	}

	jiraCommands := []cli.Command{
		jiraOpenBoard,
		jiraSearchIssue,
		jiraClaimIssue,
		{
			Name:      "create",
			Aliases:   []string{"c"},
			Usage:     "Creates a JIRA issue.",
			ArgsUsage: "SUMMARY DESCRIPTION ... [ARGS]",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "reactive", Usage: "The issue will be added to the current sprint."},
				cli.StringFlag{Name: "project", Usage: "Sets project, uses the default project is not set."},
				cli.StringFlag{Name: "transition", Usage: "Set the issue transition. e.g. Done."},
			},
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 2 {
					log.Fatal("The summary (title) and description must be passed.")
				}
				summary := c.Args().Get(0)
				desc := c.Args().Get(1)
				return atlassian.MustInitJIRA(cfg).CreateIssue(c.String("project"), summary, desc, c.String("transition"), c.Bool("reactive"))
			},
		},
		jiraOpenIssue,
		jiraTransitionIssue,
	}
	workflowCommands := []cli.Command{
		jiraOpenBoard,
		jiraClaimIssue,
		jiraOpenIssue,
		{
			Name:    "new-branch",
			Aliases: []string{"n", "new"},
			Usage:   "Checkout a new branch based on JIRA issues assigned to you.",
			Action: func(c *cli.Context) error {
				return atlassian.MustInitJIRA(cfg).CreateBranchFromAssignedIssue()
			},
		},
		{
			Name:    "checkout-branch",
			Aliases: []string{"ch", "br"},
			Usage:   "Checkout an existing branch.",
			Action: func(c *cli.Context) error {
				return core.InitGit().CheckoutBranch()
			},
		},
		{
			Name:    "commit",
			Aliases: []string{"c"},
			Usage:   "MESSAGE [OPTS]...",
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					log.Fatal("Must pass commit message.")
				}
				core.InitGit().CommitWithIssueKey(cfg, c.Args().Get(0), c.Args().Tail())
				return nil
			},
		},
		{
			Name:    "pull-request",
			Aliases: []string{"pr"},
			Usage:   "Creates a PR for the current branch.",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "t", Usage: "Transition the issue to review."},
			},
			Action: func(c *cli.Context) error {
				var title, body string
				if len(c.Args()) > 0 {
					title = c.Args().Get(0)
				}
				if len(c.Args()) > 1 {
					body = c.Args().Get(1)
				}
				return MustInitWorkflow(cfg, manifest).CreatePR(title, body, c.Bool("transition"))
			},
		},
		jiraTransitionIssue,
		{
			Name:    "log",
			Aliases: []string{"l"},
			Usage:   "Show git log and open PR, JIRA ticket, etc.",
			Action: func(c *cli.Context) error {
				return MustInitWorkflow(cfg, manifest).Log()
			},
		},
		{
			Name:    "mass",
			Aliases: []string{"m"},
			Usage:   "Mass repo changes. EXPERIMENTAL",
			Subcommands: []cli.Command{
				{
					Name:    "start",
					Aliases: []string{"s"},
					Usage:   "Clean the repository, checkout master, pull and create new branch.",
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will lose existing changes.") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassStart()
					},
				},
				{
					Name:    "done",
					Aliases: []string{"d"},
					Usage:   "Commit changes and create PRs. To be used after running '... start' and you made your changes.",
					Flags: []cli.Flag{
						cli.BoolFlag{Name: "noop", Usage: "Do not do any actions."},
					},
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will lose existing changes.") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassDone(c.Bool("noop"))
					},
				},
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "Clean the repository, checkout master and pull.",
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will lose existing changes.") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassUpdate()
					},
				},
			},
		},
	}
	circleCommands := []cli.Command{
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
	splunkCommands := []cli.Command{
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
	confluenceCommands := []cli.Command{
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
	app.Commands = []cli.Command{
		{
			Name:  "setup",
			Usage: "Setup bub on your machine.",
			Action: func(c *cli.Context) error {
				core.MustSetupConfig()
				// Reloading the config
				cfg = core.LoadConfiguration()
				aws.MustSetupConfig()
				atlassian.MustSetupJIRA(cfg)
				atlassian.MustSetupConfluence(cfg)
				integrations.MustSetupGitHub(cfg)
				ci.MustSetupJenkins(cfg)
				log.Println("Done.")
				return nil
			},
		},
		{
			Name:  "update",
			Usage: "Update the bub command to the latest release",
			Action: func(c *cli.Context) error {
				path := S3path{
					Region: cfg.Updates.Region,
					Bucket: cfg.Updates.Bucket,
					Path:   cfg.Updates.Prefix,
				}
				obj, err := latestRelease(path)
				if err != nil {
					return err
				}
				path.Path = *obj.Key
				return updateBub(path)
			},
		},
		{
			Name:  "config",
			Usage: "Edit your bub config",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "show-default", Usage: "Show default config for reference"},
			},
			Action: func(c *cli.Context) error {
				if c.Bool("show-default") {
					print(core.GetConfigString())
				} else {
					core.EditConfiguration()
				}
				return nil
			},
		},
		{
			Name:        "repository",
			Usage:       "Repository related actions",
			Aliases:     []string{"r"},
			Subcommands: repositoryCommands,
		},
		{
			Name:        "manifest",
			Aliases:     []string{"m"},
			Usage:       "Manifest related actions.",
			Subcommands: manifestCommands,
		},
		{
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
				} else if manifestErr == nil {
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
		},
		{
			Name:    "rds",
			Usage:   "RDS actions.",
			Aliases: []string{"r"},
			Action: func(c *cli.Context) error {
				aws.GetRDS(cfg).ConnectToRDSInstance(c.Args().First(), c.Args().Tail())
				return nil
			},
		},
		{
			Name:    "elasticbeanstalk",
			Usage:   "Elasticbeanstalk actions. If no sub-action specified, lists the environements.",
			Aliases: []string{"eb"},
			Flags: []cli.Flag{
				cli.StringFlag{Name: "region"},
			},
			Action: func(c *cli.Context) error {
				aws.ListEnvironments(cfg)
				return nil
			},
			Subcommands: ebCommands,
		},
		{
			Name:        "github",
			Usage:       "GitHub related actions.",
			Aliases:     []string{"gh"},
			Subcommands: gitHubCommands,
		},
		{
			Name:        "jira",
			Usage:       "JIRA related actions",
			Aliases:     []string{"ji"},
			Subcommands: jiraCommands,
		},
		{
			Name:        "workflow",
			Usage:       "Git/GitHub/JIRA workflow actions.",
			Aliases:     []string{"w"},
			Subcommands: workflowCommands,
		},
		{
			Name:        "jenkins",
			Usage:       "Jenkins related actions.",
			Aliases:     []string{"j"},
			Subcommands: jenkinsCommands,
		},
		{
			Name:        "splunk",
			Usage:       "Splunk related actions.",
			Aliases:     []string{"s"},
			Subcommands: splunkCommands,
		},
		{
			Name:        "confluence",
			Usage:       "Confluence related actions.",
			Aliases:     []string{"c"},
			Subcommands: confluenceCommands,
		},
		{
			Name:        "circle",
			Usage:       "CircleCI related actions",
			Subcommands: circleCommands,
		},
	}

	app.Run(os.Args)
}
