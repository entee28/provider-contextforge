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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// A2AAgentParameters are the configurable fields of a A2AAgent.
type A2AAgentParameters struct {
	Name        *string       `json:"name,omitempty"`
	EndpointURL string        `json:"endpointUrl"`
	Description *string       `json:"description,omitempty"`
	TeamRef     TeamReference `json:"teamRef"`
	Visibility  string        `json:"visibility"`
	Tags        []string      `json:"tags,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Capabilities *apiextensionsv1.JSON `json:"capabilities,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config             *apiextensionsv1.JSON `json:"config,omitempty"`
	PassthroughHeaders []string              `json:"passthroughHeaders,omitempty"`

	// +optional
	// +kubebuilder:default="generic"
	AgentType *string `json:"agentType,omitempty"`
	// +optional
	// +kubebuilder:default="1.0"
	ProtocolVersion *string `json:"protocolVersion,omitempty"`

	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`
}

// A2AAgentObservation are the observable fields of a A2AAgent.
type A2AAgentObservation struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
	Slug            string `json:"slug,omitempty"`
	Description     string `json:"description,omitempty"`
	EndpointURL     string `json:"endpointUrl,omitempty"`
	AgentType       string `json:"agentType,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Capabilities *apiextensionsv1.JSON `json:"capabilities,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *apiextensionsv1.JSON `json:"config,omitempty"`

	TeamID             string   `json:"teamId,omitempty"`
	Visibility         string   `json:"visibility,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	Enabled            *bool    `json:"enabled,omitempty"`
	Reachable          *bool    `json:"reachable,omitempty"`
	LastInteraction    string   `json:"lastSeen,omitempty"`
	PassthroughHeaders []string `json:"passthroughHeaders,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"`
	UpdatedAt          string   `json:"updatedAt,omitempty"`
}

// A A2AAgentSpec defines the desired state of a A2AAgent.
type A2AAgentSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              A2AAgentParameters `json:"forProvider"`
}

// A A2AAgentStatus represents the observed state of a A2AAgent.
type A2AAgentStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 A2AAgentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A A2AAgent is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,contextforge}
type ContextForgeA2AAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   A2AAgentSpec   `json:"spec"`
	Status A2AAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ContextForgeA2AAgentList contains a list of ContextForgeA2AAgent
type ContextForgeA2AAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContextForgeA2AAgent `json:"items"`
}

// A2AAgent type metadata.
var (
	A2AAgentKind             = reflect.TypeOf(ContextForgeA2AAgent{}).Name()
	A2AAgentGroupKind        = schema.GroupKind{Group: Group, Kind: A2AAgentKind}.String()
	A2AAgentKindAPIVersion   = A2AAgentKind + "." + SchemeGroupVersion.String()
	A2AAgentGroupVersionKind = SchemeGroupVersion.WithKind(A2AAgentKind)
)

func init() {
	SchemeBuilder.Register(&ContextForgeA2AAgent{}, &ContextForgeA2AAgentList{})
}
