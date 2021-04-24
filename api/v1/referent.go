package v1

// Referent contains a reference to a property in another resource.
type Referent struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	Property   string `json:"property"`
}
