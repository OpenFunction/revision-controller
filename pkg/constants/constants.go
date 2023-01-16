package constants

import "time"

const (
	RevisionType     = "type"
	PollingInterval  = "polling-interval"
	RepoType         = "repo-type"
	BaseURL          = "base-url"
	AuthType         = "auth-type"
	Project          = "project-id"
	InsecureRegistry = "insecure-registry"

	RevisionTypeSource      = "source"
	RevisionTypeSourceImage = "source-image"
	RevisionTypeImage       = "image"

	DefaultPollingInterval = time.Second * 5
)
