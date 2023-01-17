package revision

type RevisionController interface {
	Start()
	Update(config map[string]string) error
	Stop()
}
