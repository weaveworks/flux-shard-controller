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
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

var NoRequeueInterval time.Duration

// DefaultInterval is used when Interval is not specified, it
// is the default time to wait before the next reconcile loop.
const DefaultRequeueAfterSeconds = 3 * time.Minute

const FluxShardSetFinalizer = "finalizers.templates.weave.works"

// FluxShardSetSpec defines the desired state of FluxShardSet
type FluxShardSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Suspend tells the controller to suspend the reconciliation of this
	// FluxShardSet.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// Type is the type of the deployment, e.g. kustomization, helm, source, notification
	// TODO: make this an enum
	// TODO: make this required
	Type string `json:"type,omitempty"`

	// Shards is a list of shards to deploy
	Shards []ShardSpec `json:"shards,omitempty"`
}

// ShardSpec defines a shard to deploy
type ShardSpec struct {
	// Name is the name of the shard
	// TODO: make this required
	Name string `json:"name,omitempty"`
}

// FluxShardSetStatus defines the observed state of FluxShardSet
type FluxShardSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
