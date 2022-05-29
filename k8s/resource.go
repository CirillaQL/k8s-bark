package k8s

type Resource struct {
	ResourceType    string
	ResourceVersion string
	Value           interface{}
}
