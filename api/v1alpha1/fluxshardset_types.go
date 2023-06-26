/*
Copyright 2023.

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

package v1alpha1

import (
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SourceDeploymentReference struct {
	// Namespace of the referent.
	Namespace string `json:"namespace,omitempty"`
	// Name of the referent.
	Name string `json:"name"`
}

// FluxShardSetSpec defines the desired state of FluxShardSet
type FluxShardSetSpec struct {
	// Suspend tells the controller to suspend the reconciliation of this
	// FluxShardSet.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// Reference the source Deployment.
	SourceDeploymentRef SourceDeploymentReference `json:"sourceDeploymentRef"`

	// Shards is a list of shards to deploy
	Shards []ShardSpec `json:"shards,omitempty"`
}

// ShardSpec defines a shard to deploy
type ShardSpec struct {
	// Name is the name of the shard
	Name string `json:"name"`
}

// FluxShardSetStatus defines the observed state of FluxShardSet
type FluxShardSetStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// ObservedGeneration is the last observed generation of the HelmRepository
	// object.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the FluxShardSet
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Inventory contains the list of Kubernetes resource object references that
	// have been successfully applied
	// +optional
	Inventory *ResourceInventory `json:"inventory,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// FluxShardSet is the Schema for the fluxshardsets API
type FluxShardSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluxShardSetSpec   `json:"spec,omitempty"`
	Status FluxShardSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FluxShardSetList contains a list of FluxShardSet
type FluxShardSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxShardSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluxShardSet{}, &FluxShardSetList{})
}
