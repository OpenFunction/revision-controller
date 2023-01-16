package revision

type Revision interface {
	Start()
	Update(config map[string]string) error
	Stop()
}
