package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Referent contains a reference to a property in another resource.
type Referent struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	Property   string `json:"property"`
}

// SyncerSpec defines the desired state of a Syncer
type SyncerSpec struct {
	Source Referent `json:"source"`
	Target Referent `json:"target"`
}

// SyncerStatus defines the observed state of a Syncer
type SyncerStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Syncer is the Schema for the Syncer API
type Syncer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncerSpec   `json:"spec,omitempty"`
	Status SyncerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SyncerList contains a list of Syncer objects
type SyncerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Syncer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Syncer{}, &SyncerList{})
}
