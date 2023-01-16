package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v49/github"
	"github.com/openfunction/revision-controller/pkg/revision/git/provider"
	"golang.org/x/oauth2"
)

type Provider struct {
	config *provider.GitConfig
	client *github.Client

	owner  string
	repo   string
	branch string
}

func NewProvider(config *provider.GitConfig) (provider.GitProvider, error) {
	p := &Provider{
		config: config,
		client: github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config.Password},
		))),
	}

	url := config.URL
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimSuffix(url, ".git")
	paths := strings.Split(url, "/")
	p.owner = paths[0]
	p.repo = paths[1]

	if config.Branch == nil || *config.Branch == "" {
		repository, resp, err := p.client.Repositories.Get(context.Background(), p.owner, p.repo)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s", resp.Status)
		}

		if repository.DefaultBranch == nil {
			return nil, fmt.Errorf("%s", "unknown default branch")
		}

		p.branch = *repository.DefaultBranch
	} else {
		p.branch = *config.Branch
	}

	_, resp, err := p.client.Repositories.GetBranch(context.Background(), p.owner, p.repo, p.branch, true)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get branch error, %s", resp.Status)
	}

	return p, nil
}

func (p *Provider) GetHead() (string, error) {
	commits, resp, err := p.client.Repositories.ListCommits(context.Background(), p.owner, p.repo, &github.CommitsListOptions{
		SHA: p.branch,
		ListOptions: github.ListOptions{
			PerPage: 1,
		},
	})
	if err != nil {
		return "", err
	}

	if resp != nil && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", resp.Status)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("%s", "no commit found")
	}

	return *commits[0].SHA, nil
}
