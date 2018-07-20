package main

// ObjectResource is minimal kubernetes resource representation
type ObjectResource struct {
	Kind             string `yaml:"kind"`
	ObjectMeta       `yaml:"metadata,omitempty"`
	Template         []byte `yaml:"-"`
	FileName         string `yaml:"-"`
	DeploymentStatus `yaml:"status,omitempty"`
	ObjectSpec       `yaml:"spec"`
	CreateOnly       bool `yaml:"-"`
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

	// GenerateName causes kubernetes to generate a random resource name for you on create, it takes the given string and suffixes a random string to it
	GenerateName string `yaml:"generateName,omitempty"`
}

// DeploymentStatus is the most recently observed status of the Deployment / Statefulset / DaemonSets.
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

	// ReadyReplicas is the number of Pods created by the StatefulSet controller that have a Ready Condition.
	ReadyReplicas int32 `yaml:"readyReplicas,omitempty"`

	// CurrentRevision is the last revision completely deployed before any updates
	CurrentRevision string `yaml:"currentRevision,omitempty"`

	// UpdateRevision is the version currently being deployed. Will match CurrentRevision when complete.
	UpdateRevision string `yaml:"updateRevision,omitempty"`

	// Start: Daemonset statuses
	// The number of nodes that are running at least 1 daemon pod and are supposed to run the daemon pod
	CurrentNumberScheduled int32 `yaml:"currentNumberScheduled,omitempty"`

	// The total number of nodes that should be running the daemon pod (including nodes correctly running the daemon pod)
	DesiredNumberScheduled int32 `yaml:"desiredNumberScheduled,omitempty"`

	// The number of nodes that should be running the daemon pod and have one or more of the daemon pod running and available (ready for at least spec.minReadySeconds)
	NumberAvailable int32 `yaml:"numberAvailable,omitempty"`

	// The number of nodes that are running the daemon pod, but are not supposed to run the daemon pod
	NumberMisscheduled int32 `yaml:"numberMisscheduled,omitempty"`

	// The number of nodes that should be running the daemon pod and have one or more of the daemon pod running and ready.
	NumberReady int32 `yaml:"numberReady,omitempty"`

	// The number of nodes that should be running the daemon pod and have none of the daemon pod running and available (ready for at least spec.minReadySeconds)
	NumberUnavailable int32 `yaml:"numberUnavailable,omitempty"`

	// The total number of nodes that are running updated daemon pod
	UpdatedNumberScheduled int32 `yaml:"updatedNumberScheduled,omitempty"`
	// End: Daemonset statuses

	// Job Succeeded status
	Succeeded int32 `yaml:"succeeded,omitempty"`
}

// ObjectSpec - fields used for setting StatefulSet update behaviour
type ObjectSpec struct {
	// UpdateStrategy indicates the StatefulSetUpdateStrategy that will be employed to update Pods in the StatefulSet when a revision is made to Template.
	UpdateStrategy `yaml:"updateStrategy,omitempty"`

	// Replicas indicates how many intended pods are required for a StatefulSet
	Replicas int32 `yaml:"replicas,omitempty"`
}

// UpdateStrategy indicates the StatefulSetUpdateStrategy that will be employed to update Pods in the StatefulSet when a revision is made to Template.
type UpdateStrategy struct {
	// Type is the choosen UpdateStrategy which can be RollingUpdate
	Type string `yaml:"type,omitempty"`
}
