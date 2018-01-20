package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/dep/gps"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func msg(err error, errStream io.Writer) int {
	if err != nil {
		fmt.Fprintf(errStream, "%s: %v\n", os.Args[0], err)
		return 1
	}
	return 0
}

func run(args []string, outStream, errStream io.Writer) int {
	app := cli.NewApp()
	app.Name = "ci-dep-update"
	app.Usage = "create a dep update PR"
	app.Version = "0.1.0"
	app.Flags = commandFlags()
	app.Action = appRun

	return msg(app.Run(args), outStream)
}

func appRun(c *cli.Context) error {
	var err error
	var result bool

	if err = checkRequiredArguments(c); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	beforeLock, _ := readLock("Gopkg.lock")

	if err = runDepUpdate(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if result, err = isNeedUpdate(); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	if !result {
		return nil
	}

	ctx := context.Background()
	token := c.String("github_access_token")
	client := gitHubClient(token, &ctx)

	user := c.String("user")
	repo := c.String("repository")
	email := c.String("email")
	if len(email) == 0 {
		email = user + "@users.noreply.github.com"
	}
	branch := "dep-update-" + time.Now().Format("2006-01-02-150405")
	afterLock, _ := readLock("Gopkg.lock")
	lockDiff := gps.DiffLocks(beforeLock, afterLock)

	createBranchAndCommit(user, email, token, repo, branch)
	if err = createPullRequest(&ctx, client, lockDiff, repo, branch); err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}

func commandFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "github_access_token",
			Value:  "",
			Usage:  "GitHub access token",
			EnvVar: "GITHUB_ACCESS_TOKEN",
		},
		cli.StringFlag{
			Name:   "user, u",
			Value:  "",
			Usage:  "Git user name",
			EnvVar: "GIT_USER_NAME",
		},
		cli.StringFlag{
			Name:   "email, e",
			Value:  "",
			Usage:  "Git user email",
			EnvVar: "GIT_USER_EMAIL",
		},
		cli.StringFlag{
			Name:   "repository, r",
			Value:  "",
			Usage:  "Repository url",
			EnvVar: "REPOSITORY_URL",
		},
	}
}

func checkRequiredArguments(c *cli.Context) error {
	if c.String("user") == "" {
		return errors.New("please set Git user name")
	}
	if c.String("repository") == "" {
		return errors.New("please set repository URL")
	}
	if c.String("github_access_token") == "" {
		return errors.New("please set GitHub access token")
	}

	return nil
}

func runDepUpdate() error {
	if stdoutStederr, err := exec.Command("dep", "ensure", "-update").CombinedOutput(); err != nil {
		return errors.New("run dep failed. cause: " + string(stdoutStederr))
	}
	return nil
}

func createBranchAndCommit(username, useremail, token, repo, branch string) {
	remote := "https://" + token + "@github.com/" + repo
	exec.Command("git", "remote", "add", "github-url-with-token", remote).Run()
	exec.Command("git", "checkout", "-b", branch).Run()
	exec.Command("git", "config", "user.name", username).Run()
	exec.Command("git", "config", "user.email", useremail).Run()
	exec.Command("git", "add", "Gopkg.lock").Run()
	exec.Command("git", "commit", "-m", "Run 'dep ensure -update'").Run()
	exec.Command("git", "push", "-q", "github-url-with-token", branch).Run()
}

func isNeedUpdate() (bool, error) {
	output, err := exec.Command("git", "diff", "--name-only").Output()
	if err != nil {
		return false, errors.Wrap(err, "git diff")
	}

	result := strings.Contains(string(output), "Gopkg.lock")
	return result, nil
}

func gitHubClient(accessToken string, ctx *context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(*ctx, ts)
	return github.NewClient(tc)
}

func createPullRequest(ctx *context.Context, client *github.Client, lockDiff *gps.LockDiff, repo, branch string) error {
	title := github.String("Dep update at " + time.Now().Format("2006-01-02 15:04:05"))
	body := github.String(generatePullRequestBody(lockDiff))
	base := github.String("master")
	ownerAndRepo := strings.Split(repo, "/")
	head := github.String(ownerAndRepo[0] + ":" + branch)
	pr := &github.NewPullRequest{Title: title, Head: head, Base: base, Body: body}

	_, _, err := client.PullRequests.Create(*ctx, ownerAndRepo[0], ownerAndRepo[1], pr)
	if err != nil {
		return err
	}
	return nil
}

func generatePullRequestBody(diff *gps.LockDiff) string {
	result := "**Changed:**\n\n"
	for _, prj := range diff.Modify {
		result += generateDiffLink(&prj)
	}

	return result
}

func generateDiffLink(prj *gps.LockedProjectDiff) string {
	var compareLink string
	name := string(prj.Name)

	prev := prj.Revision.Previous[:7]
	cur := prj.Revision.Current[:7]

	if prj.Version != nil {
		prev = prj.Version.Previous
		cur = prj.Version.Current
	}

	if strings.Contains(name, "github.com") {
		compareLink = fmt.Sprintf("[%s...%s](https://%s/compare/%s...%s)", prev, cur, prj.Name, prev, cur)
		return fmt.Sprintf("* [%s](https://%s) %s\n", prj.Name, prj.Name, compareLink)
	}
	return fmt.Sprintf("* [%s](https://%s) %s...%s\n", prj.Name, prj.Name, prev, cur)
}
