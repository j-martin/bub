package main

import (
	"github.com/manifoldco/promptui"
	"github.com/tmc/keyring"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"strings"
)

type RDSConfiguration struct {
	Prefix, Database, User, Password string
}

type Environment struct {
	Prefix, Jumphost, Region string
}

type User struct {
	Name, Slack string
}

type Configuration struct {
	AWS struct {
		Regions      []string
		RDS          []RDSConfiguration
		Environments []Environment
	}
	GitHub struct {
		Organization, Token string
		Reviewers           []string
	}
	Users   []User
	JIRA    ServiceConfiguration
	Jenkins ServiceConfiguration
	Splunk  struct {
		Server string
	}
	Confluence ServiceConfiguration
	Circle     struct {
		Token string
	}
	Updates struct {
		Region, Bucket, Prefix string
	}
	Ssh struct {
		ConnectTimeout uint `yaml:"connectTimeout"`
	}
}

type ServiceConfiguration struct {
	Server, Username, Password string
}

var config = `---
aws:
	regions:
		- us-east-1
		- us-west-2

	rds:
		# The first prefix match will be used.
		# The database name, unless specified, will be inferred from the host name.
		- prefix: staging
			database: <optional>
			user: <optional>
			password: <optional>

	environments:
		- prefix: staging2
			jumphost: jump.staging2.example.com
			region: us-west-2
		- prefix: staging
			jumphost: jump.example.com
			region: us-west-2
		# if not prefix, act as a catch all.
		- jumphost: jump.example.com
			region: us-east-1

github:
	organization: benchlabs
	reviewers:
		# - reviewers (GitHub username) that will be applied to the PRs by default.

jenkins:
	server: "https://jenkins.example.com"

confluence:
	server: "https://example.atlassian.net/wiki"

jira:
	server: "https://example.atlassian.net"

splunk:
	server: "https://splunk.example.com"

circle:
	token: <optional-change-me>

updates:
	region: us-east-1
	bucket: s3bucket
	prefix: contrib/bub

ssh:
	connectTimeout: 3
`

func GetConfigString() string {
	return strings.Replace(config, "\t", "  ", -1)
}

func loadConfiguration() *Configuration {
	cfg := Configuration{}

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	configDir := path.Join(usr.HomeDir, ".config", "bub")
	configPath := path.Join(configDir, "config.yml")

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Print("No bub configuration found. Please run `bub setup`")
		return &cfg
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		log.Printf("Could not parse yaml file. %v", err)
		return &cfg
	}

	if len(cfg.AWS.Regions) == 0 {
		cfg.AWS.Regions = []string{"us-east-1", "us-west-2"}
	}

	return &cfg
}

func editConfiguration() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	configPath := path.Join(usr.HomeDir, ".config", "bub", "config.yml")
	createAndEdit(configPath, GetConfigString())
}

func setup() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	//TODO: move aws configuration to config.yml
	awsCredentials := `[default]
output=json
region=us-east-1
aws_access_key_id = CHANGE_ME
aws_secret_access_key = CHANGE_ME`

	createAndEdit(path.Join(usr.HomeDir, ".aws", "credentials"), awsCredentials)
	createAndEdit(path.Join(usr.HomeDir, ".config", "bub", "config.yml"), GetConfigString())

	log.Println("Done.")
}

func createAndEdit(filePath string, content string) {
	directory := path.Dir(filePath)
	log.Print(directory)
	dirExists, err := pathExists(directory)
	if err != nil {
		log.Fatal(err)
	}

	if !dirExists {
		os.MkdirAll(directory, 0700)
	}

	fileExists, err := pathExists(filePath)
	if err != nil {
		log.Fatal(err)
	}

	if !fileExists {
		log.Printf("Creating %s file.", filePath)
		ioutil.WriteFile(filePath, []byte(content), 0700)
	}

	log.Printf("Editing %s.", filePath)
	editFile(filePath)
}

func checkServerConfig(cfg ServiceConfiguration) {
	if cfg.Server == "" {
		log.Fatal("Server cannot be empty, make sure the config file is properly configured. run 'bub config'.")
	}
}

func loadCredentials(item string, cfg *ServiceConfiguration) (err error) {
	if err = loadCredentialItem(item+" Username", &cfg.Username); err != nil {
		return err
	}
	if err = loadCredentialItem(item+" Password", &cfg.Password); err != nil {
		return err
	}
	return nil
}

func loadCredentialItem(item string, ptr *string) (err error) {
	if strings.ToLower(os.Getenv("BUB_RESET_PASSWORD")) == "true" {
		return setKeyringItem(item, ptr)
	}
	// e.g. "Confluence Username" -> "CONFLUENCE_USERNAME"
	envVar := os.Getenv(strings.Replace(strings.ToUpper(item), " ", "_", -1))
	if envVar != "" {
		*ptr = envVar
		return
	}

	if *ptr != "" && !strings.HasPrefix(*ptr, "<optional-") {
		return nil
	}

	return loadKeyringItem(item, ptr)
}

func loadKeyringItem(item string, ptr *string) (err error) {
	service := "bub"
	if pw, err := keyring.Get(service, item); err == nil {
		*ptr = pw
		return nil
	} else if err == keyring.ErrNotFound {
		return setKeyringItem(item, ptr)
	} else {
		return err
	}
}

func setKeyringItem(item string, ptr *string) (err error) {
	service := "bub"
	prompt := promptui.Prompt{
		Label: "Enter " + item,
	}
	if strings.HasSuffix(strings.ToLower(item), "password") {
		prompt.Mask = '*'
	}
	result, err := prompt.Run()
	if err != nil {
		return err
	}
	err = keyring.Set(service, item, string(result))
	if err != nil {
		return err
	}
	return loadKeyringItem(item, ptr)
}
