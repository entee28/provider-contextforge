/*
Copyright 2025 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// TeamParameters are the configurable fields of a ContextForgeTeam.
type TeamParameters struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="name is immutable"
	Name string `json:"name"`
	// +optional
	Description *string `json:"description,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxMembers *int `json:"maxMembers,omitempty"`
}

// TeamObservation are the observable fields of a ContextForgeTeam.
type TeamObservation struct {
	ID          string `json:"id,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Name        string `json:"name,omitempty"`
	MaxMembers  *int   `json:"maxMembers,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
	IsActive    *bool  `json:"isActive,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// A TeamSpec defines the desired state of a ContextForgeTeam.
type TeamSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              TeamParameters `json:"forProvider"`
}

// A TeamStatus represents the observed state of a ContextForgeTeam.
type TeamStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 TeamObservation `json:"atProvider,omitempty"`
}

// TeamReference identifies a ContextForgeTeam resource in the same namespace.
type TeamReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// +kubebuilder:object:root=true

// A ContextForgeTeam represents a ContextForge team.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,contextforge}
type ContextForgeTeam struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeamSpec   `json:"spec"`
	Status TeamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContextForgeTeamList contains a list of ContextForgeTeam
type ContextForgeTeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContextForgeTeam `json:"items"`
}

// ContextForgeTeam type metadata.
var (
	TeamKind             = reflect.TypeOf(ContextForgeTeam{}).Name()
	TeamGroupKind        = schema.GroupKind{Group: Group, Kind: TeamKind}.String()
	TeamKindAPIVersion   = TeamKind + "." + SchemeGroupVersion.String()
	TeamGroupVersionKind = SchemeGroupVersion.WithKind(TeamKind)
)

func init() {
	SchemeBuilder.Register(&ContextForgeTeam{}, &ContextForgeTeamList{})
}
