package revision_controller

type RevisionController interface {
	Start()
	Update(config map[string]string) error
	Stop()
}
