package main

// K8Api is an abstraction to allow the migration to the real API not kubectl
type K8Api interface {
	// Lookup abstract interface for finding kuberneets api data by kind, name and path
	Lookup(kind, name, path string) (string, error)
}
