package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/golang/dep/gps"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"
)

type DepUpdater struct {
	cli *cli.Context
}

func NewDepUpdater(cli *cli.Context) *DepUpdater {
	updater := &DepUpdater{cli: cli}
	return updater
}

func (updater *DepUpdater) Run() error {
	var result bool

	beforeLock, err := readLock("Gopkg.lock")
	if err != nil {
		return err
	}

	if err = updater.runDepUpdate(); err != nil {
		return err
	}

	if result, err = updater.isNeedUpdate(); err != nil {
		return err
	}

	if !result {
		return nil
	}

	ctx := context.Background()
	token := updater.cli.String("github_access_token")
	client := updater.gitHubClient(token, &ctx)

	user := updater.cli.String("user")
	repo := updater.cli.String("repository")
	email := updater.cli.String("email")
	if len(email) == 0 {
		email = user + "@users.noreply.github.com"
	}

	branch := "dep-update-" + time.Now().Format("2006-01-02-150405")
	afterLock, _ := readLock("Gopkg.lock")
	lockDiff := gps.DiffLocks(beforeLock, afterLock)

	updater.createBranchAndCommit(user, email, token, repo, branch)
	if err = updater.createPullRequest(&ctx, client, lockDiff, repo, branch); err != nil {
		return err
	}

	return nil
}

func (updater *DepUpdater) runDepUpdate() error {
	if stdoutStederr, err := exec.Command("dep", "ensure", "-update").CombinedOutput(); err != nil {
		return errors.New("run dep failed. cause: " + string(stdoutStederr))
	}
	return nil
}

func (updater *DepUpdater) isNeedUpdate() (bool, error) {
	output, err := exec.Command("git", "diff", "--name-only").Output()
	if err != nil {
		return false, errors.Wrap(err, "git diff")
	}

	result := strings.Contains(string(output), "Gopkg.lock")
	return result, nil
}

func (updater *DepUpdater) createBranchAndCommit(username, useremail, token, repo, branch string) {
	remote := "https://" + token + "@github.com/" + repo
	exec.Command("git", "remote", "add", "github-url-with-token", remote).Run()
	exec.Command("git", "checkout", "-b", branch).Run()
	exec.Command("git", "config", "user.name", username).Run()
	exec.Command("git", "config", "user.email", useremail).Run()
	exec.Command("git", "add", "Gopkg.lock").Run()
	exec.Command("git", "commit", "-m", "Run 'dep ensure -update'").Run()
	exec.Command("git", "push", "-q", "github-url-with-token", branch).Run()
}

func (updater *DepUpdater) gitHubClient(accessToken string, ctx *context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	tc := oauth2.NewClient(*ctx, ts)
	return github.NewClient(tc)
}

func (updater *DepUpdater) createPullRequest(ctx *context.Context, client *github.Client, lockDiff *gps.LockDiff, repo, branch string) error {
	title := github.String("Dep update at " + time.Now().Format("2006-01-02 15:04:05"))
	body := github.String(updater.generatePullRequestBody(lockDiff))
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

func (updater *DepUpdater) generatePullRequestBody(diff *gps.LockDiff) string {
	result := "**Changed:**\n\n"
	for _, prj := range diff.Modify {
		result += updater.generateDiffLink(&prj)
	}

	return result
}

func (updater *DepUpdater) generateDiffLink(prj *gps.LockedProjectDiff) string {
	var compareLink string
	var pkg string
	var url string
	name := string(prj.Name)
	golangOrg := "golang.org/x/"
	golangOrgLen := len(golangOrg)

	prev := prj.Revision.Previous[:7]
	cur := prj.Revision.Current[:7]

	if prj.Version != nil {
		prev = prj.Version.Previous
		cur = prj.Version.Current
	}

	if strings.Contains(name, "github.com") {
		compareLink = fmt.Sprintf("[%s...%s](https://%s/compare/%s...%s)", prev, cur, name, prev, cur)
		return fmt.Sprintf("* [%s](https://%s) %s\n", name, name, compareLink)
	} else if name[:golangOrgLen] == golangOrg {
		pkg = name[golangOrgLen:]
		url = "https://github.com/golang/" + pkg
		return fmt.Sprintf("* [%s](%s) [%s...%s](%s/compare/%s...%s)\n", name, url, prev, cur, url, prev, cur)
	}
	return fmt.Sprintf("* [%s](https://%s) %s...%s\n", prj.Name, prj.Name, prev, cur)
}
