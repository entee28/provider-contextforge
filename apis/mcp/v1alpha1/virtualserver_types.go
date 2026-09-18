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

	resource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// interface checks to ensure our types conform to the crossplane-runtime interfaces
var (
	_ resource.ModernManaged = &ContextForgeVirtualServer{}
	_ resource.ManagedList   = &ContextForgeVirtualServerList{}
)

// VirtualServerParameters are the configurable fields of a ContextForgeVirtualServer.
type VirtualServerParameters struct {
	// +optional
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_.\- ]+$`
	Name *string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="teamRef is immutable"
	TeamRef TeamReference `json:"teamRef"`

	// +kubebuilder:validation:Required
	// "private" is deliberately excluded. ContextForge scopes private
	// visibility to the creating identity's owner_email, but this provider
	// always authenticates as one shared service account - a private
	// resource would be invisible to the tenant who created it and their
	// team, not just restricted.
	// +kubebuilder:validation:Enum=team;public
	Visibility string `json:"visibility"`

	// +optional
	Description *string `json:"description,omitempty"`

	// +optional
	Icon *string `json:"icon,omitempty"`

	// +optional
	Tags []string `json:"tags,omitempty"`

	// +optional
	AssociatedTools []string `json:"associatedTools,omitempty"`
	// +optional
	AssociatedResources []string `json:"associatedResources,omitempty"`
	// +optional
	AssociatedPrompts []string `json:"associatedPrompts,omitempty"`
	// +optional
	AssociatedA2AAgents []string `json:"associatedA2aAgents,omitempty"`
}

// VirtualServerObservation are the observable fields of a ContextForgeVirtualServer.
type VirtualServerObservation struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	TeamID     string   `json:"teamId,omitempty"`
	OwnerEmail string   `json:"ownerEmail,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	CreatedAt  string   `json:"createdAt,omitempty"`
	UpdatedAt  string   `json:"updatedAt,omitempty"`
}

// A VirtualServerSpec defines the desired state of a ContextForgeVirtualServer.
type VirtualServerSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              VirtualServerParameters `json:"forProvider"`
}

// A VirtualServerStatus represents the observed state of a ContextForgeVirtualServer.
type VirtualServerStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 VirtualServerObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A ContextForgeVirtualServer represents a ContextForge virtual server.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,contextforge}
type ContextForgeVirtualServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualServerSpec   `json:"spec"`
	Status VirtualServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContextForgeVirtualServerList contains a list of ContextForgeVirtualServer
type ContextForgeVirtualServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContextForgeVirtualServer `json:"items"`
}

// ContextForgeVirtualServer type metadata.
var (
	VirtualServerKind             = reflect.TypeOf(ContextForgeVirtualServer{}).Name()
	VirtualServerGroupKind        = schema.GroupKind{Group: Group, Kind: VirtualServerKind}.String()
	VirtualServerKindAPIVersion   = VirtualServerKind + "." + SchemeGroupVersion.String()
	VirtualServerGroupVersionKind = SchemeGroupVersion.WithKind(VirtualServerKind)
)

func init() {
	SchemeBuilder.Register(&ContextForgeVirtualServer{}, &ContextForgeVirtualServerList{})
}
