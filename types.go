package main

// ObjectResource is minimal kubernetes resource representation
type ObjectResource struct {
	Kind             string `yaml:"kind"`
	ObjectMeta       `yaml:"metadata,omitempty"`
	Template         []byte `yaml:"-"`
	FileName         string `yaml:"-"`
	DeploymentStatus `yaml:"status,omitempty"`
}

// ObjectMeta is a resource metadata that all persisted resources must have
type ObjectMeta struct {
	// Name is unique within a namespace.  Name is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	Name string `yaml:"name,omitempty"`

	// Namespace defines the space within which name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	Namespace string `yaml:"namespace,omitempty"`
}

// DeploymentStatus is the most recently observed status of the Deployment.
type DeploymentStatus struct {
	// The generation observed by the deployment controller.
	ObservedGeneration int64 `yaml:"observedGeneration,omitempty"`

	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	Replicas int32 `yaml:"replicas,omitempty"`

	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	UpdatedReplicas int32 `yaml:"updatedReplicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	AvailableReplicas int32 `yaml:"availableReplicas,omitempty"`

	// Total number of unavailable pods targeted by this deployment.
	UnavailableReplicas int32 `yaml:"unavailableReplicas,omitempty"`
}
