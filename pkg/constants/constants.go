package constants

import "time"

const (
	RevisionControllerType = "type"
	PollingInterval        = "polling-interval"
	RepoType               = "repo-type"
	BaseURL                = "base-url"
	AuthType               = "auth-type"
	Project                = "project-id"
	InsecureRegistry       = "insecure-registry"

	RevisionControllerTypeSource      = "source"
	RevisionControllerTypeSourceImage = "source-image"
	RevisionControllerTypeImage       = "image"

	DefaultPollingInterval = time.Second * 5
)
