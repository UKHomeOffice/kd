package main

// K8ApiNoop is a noop API runner used when not connected to a server
type K8ApiNoop struct {
	K8Api
}

// NewK8ApiNoop creats a new K8Api implimentaion based on K8ApiNoop
func NewK8ApiNoop() K8Api {
	return &K8ApiNoop{}
}

// Lookup will pretentd to get data from a specified kubernetes object
func (a K8ApiNoop) Lookup(kind, name, path string) (string, error) {
	return "noop", nil
}
