/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncBindingSpec defines the desired state of SyncBinding
type SyncBindingSpec struct {
	Source   Referent `json:"source"`
	Target   Referent `json:"target"`
	Interval string   `json:"interval"`
}

// SyncBindingStatus defines the observed state of SyncBinding
type SyncBindingStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SyncBinding is the Schema for the syncbindings API
type SyncBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncBindingSpec   `json:"spec,omitempty"`
	Status SyncBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SyncBindingList contains a list of SyncBinding
type SyncBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncBinding{}, &SyncBindingList{})
}
