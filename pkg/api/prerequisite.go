package api

type TestPrerequisite interface {
	Setup() error
	Destroy() error
}
