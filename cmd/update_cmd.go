package cmd

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/j-martin/bub/core"
	"github.com/j-martin/bub/integrations/aws"
	"github.com/j-martin/bub/utils"
	"github.com/mcuadros/go-version"
	"github.com/urfave/cli"
)

type s3path struct {
	Region, Bucket, Path string
}

func buildUpdateCmd(cfg *core.Configuration) cli.Command {
	return cli.Command{
		Name:  "update",
		Usage: "Update the bub command to the latest release.",
		Action: func(c *cli.Context) error {
			path := s3path{
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
	}
}

func latestRelease(base s3path) (obj *s3.Object, err error) {
	s3cfg := aws.GetAWSConfig(base.Region)
	sess, err := session.NewSession(&s3cfg)
	if err != nil {
		return nil, err
	}
	svc := s3.New(sess)
	results, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: &base.Bucket,
		Prefix: &base.Path,
	})
	if err != nil {
		return nil, err
	}

	// regex to find version numbers in pathnames
	versionRegex, err := regexp.Compile("[0-9\\.]+")
	if err != nil {
		return nil, err
	}

	newestVersion := "0.0"
	var newestObj *s3.Object
	for _, obj := range results.Contents {
		if strings.Contains(*obj.Key, runtime.GOOS) {
			currentVersion := string(versionRegex.Find([]byte(*obj.Key)))
			if version.CompareSimple(currentVersion, newestVersion) > 0 {
				newestVersion = currentVersion
				newestObj = obj
			}
		}
	}
	return newestObj, nil
}

func updateBub(path s3path) error {
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Could not get bub's path: %s", err)
	}
	log.Printf("Downloading s3://%s/%s to %s", path.Bucket, path.Path, exe)
	s3cfg := aws.GetAWSConfig(path.Region)
	sess, err := session.NewSession(&s3cfg)
	if err != nil {
		return err
	}
	downloader := s3manager.NewDownloader(sess)

	// dl gzipped upstream content to temp file
	fgz, err := ioutil.TempFile("", "bub-update")
	if err != nil {
		return err
	}
	defer fgz.Close()
	defer os.Remove(fgz.Name())
	_, err = downloader.Download(fgz, &s3.GetObjectInput{
		Bucket: &path.Bucket,
		Key:    &path.Path,
	})
	if err != nil {
		return err
	}

	// uncompress to second tempfile
	_, err = fgz.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	f, err := ioutil.TempFile("", "bub-update")
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	// transparently gunzip as we download
	gzr, err := gzip.NewReader(fgz)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, gzr)
	if err != nil {
		return err
	}
	if err = os.Chmod(f.Name(), 0755); err != nil {
		return err
	}

	if os.Rename(f.Name(), exe) != nil {
		return err
	}
	log.Printf("Update complete.")
	newVersion, err := utils.RunCmdWithStdout(exe, "--version")
	if err != nil {
		return err
	}

	log.Printf("New version: %v", newVersion)
	return nil
}
