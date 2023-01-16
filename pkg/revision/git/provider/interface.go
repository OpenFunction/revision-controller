package provider

type GitProvider interface {
	GetHead() (string, error)
}

type GitConfig struct {
	URL      string
	Branch   *string
	Password string
	AuthType string
	BaseURL  string
	Project  string
}
