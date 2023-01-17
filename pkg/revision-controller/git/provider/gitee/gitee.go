package gitee

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
	"github.com/openfunction/revision-controller/pkg/revision-controller/git/provider"
	"golang.org/x/oauth2"
)

type Provider struct {
	config *provider.GitConfig
	client *gitee.APIClient

	owner  string
	repo   string
	branch string
}

func NewProvider(config *provider.GitConfig) (provider.GitProvider, error) {
	p := &Provider{
		config: config,
	}

	conf := gitee.NewConfiguration()
	conf.HTTPClient = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Password},
	))
	p.client = gitee.NewAPIClient(conf)

	url := config.URL
	url = strings.TrimPrefix(url, "https://gitee.com/")
	url = strings.TrimSuffix(url, ".git")
	paths := strings.Split(url, "/")
	p.owner = paths[0]
	p.repo = paths[1]

	if config.Branch == nil || *config.Branch == "" {
		project, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepo(context.Background(), p.owner, p.repo, &gitee.GetV5ReposOwnerRepoOpts{})
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s", resp.Status)
		}

		p.branch = project.DefaultBranch
	} else {
		p.branch = *config.Branch
	}

	_, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepoBranchesBranch(context.Background(), p.owner, p.repo, p.branch, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get branch error, %s", resp.Status)
	}

	return p, nil
}

func (p *Provider) GetHead() (string, error) {
	commits, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepoCommits(context.Background(), p.owner, p.repo, &gitee.GetV5ReposOwnerRepoCommitsOpts{
		Sha:     optional.NewString(p.branch),
		PerPage: optional.NewInt32(1),
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

	return commits[0].Sha, nil
}
