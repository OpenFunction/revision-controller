package gitlab

import (
	"fmt"
	"net/http"

	"github.com/openfunction/revision-controller/pkg/revision-controller/git/provider"
	"github.com/xanzy/go-gitlab"
)

const (
	jobTokenType     = "JobToken"
	oauthTokenType   = "OAuthToken"
	privateTokenType = "PrivateToken"
)

type Provider struct {
	config *provider.GitConfig
	client *gitlab.Client

	branch string
}

func NewProvider(config *provider.GitConfig) (provider.GitProvider, error) {
	p := &Provider{
		config: config,
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("base url must be specified")
	}

	if config.Project == "" {
		return nil, fmt.Errorf("project id must be specified")
	}

	authType := config.AuthType
	if authType == "" {
		authType = privateTokenType
	}
	var err error
	switch authType {
	case jobTokenType:
		p.client, err = gitlab.NewJobClient(config.Password, gitlab.WithBaseURL(config.BaseURL))
	case oauthTokenType:
		p.client, err = gitlab.NewOAuthClient(config.Password, gitlab.WithBaseURL(config.BaseURL))
	case privateTokenType:
		p.client, err = gitlab.NewClient(config.Password, gitlab.WithBaseURL(config.BaseURL))
	default:
		return nil, fmt.Errorf("unspport auth type, %s", authType)
	}
	if err != nil {
		return nil, err
	}

	if config.Branch == nil || *config.Branch == "" {
		repository, resp, err := p.client.Projects.GetProject(p.config.Project, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s", resp.Status)
		}

		p.branch = repository.DefaultBranch
	} else {
		p.branch = *config.Branch
	}

	_, resp, err := p.client.Branches.GetBranch(p.config.Project, p.branch)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get branch error, %s", resp.Status)
	}

	return p, nil
}

func (p *Provider) GetHead() (string, error) {
	ref := p.branch
	commits, resp, err := p.client.Commits.ListCommits(p.config.Project, &gitlab.ListCommitsOptions{
		RefName: &ref,
		ListOptions: gitlab.ListOptions{
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

	return commits[0].ID, nil
}
