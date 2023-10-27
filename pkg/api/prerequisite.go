package api

type TestPrerequisite interface {
	Setup() error
	Destroy() error
}

type Readiness interface {
	IsReady() (bool, error)
}
