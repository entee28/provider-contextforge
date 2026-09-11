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

// GatewayParameters are the configurable fields of a Gateway.
type GatewayParameters struct {
	Name        *string `json:"name,omitempty"`
	URL         string  `json:"url"`
	Description *string `json:"description,omitempty"`
	// +kubebuilder:validation:Enum=SSE;STREAMABLEHTTP
	Transport  string        `json:"transport"`
	TeamRef    TeamReference `json:"teamRef"`
	Visibility string        `json:"visibility"`
	Tags       []string      `json:"tags,omitempty"`

	// +optional
	Auth *AuthConfig `json:"auth,omitempty"`
}

// GatewayObservation are the observable fields of a Gateway.
type GatewayObservation struct {
	ID                   string   `json:"id,omitempty"`
	Name                 string   `json:"name,omitempty"`
	Slug                 string   `json:"slug,omitempty"`
	URL                  string   `json:"url,omitempty"`
	Transport            string   `json:"transport,omitempty"`
	TeamID               string   `json:"teamId,omitempty"`
	Visibility           string   `json:"visibility,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	GatewayMode          string   `json:"gatewayMode,omitempty"`
	Enabled              *bool    `json:"enabled,omitempty"`
	Reachable            *bool    `json:"reachable,omitempty"`
	Status               string   `json:"status,omitempty"`
	StatusMessage        string   `json:"statusMessage,omitempty"`
	LastError            string   `json:"lastError,omitempty"`
	RegistrationAttempts int      `json:"registrationAttempts,omitempty"`
	NextRetryAt          string   `json:"nextRetryAt,omitempty"`
	LastSeen             string   `json:"lastSeen,omitempty"`
	ToolCount            int      `json:"toolCount,omitempty"`
	CreatedAt            string   `json:"createdAt,omitempty"`
	UpdatedAt            string   `json:"updatedAt,omitempty"`
}

// A GatewaySpec defines the desired state of a Gateway.
type GatewaySpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              GatewayParameters `json:"forProvider"`
}

// A GatewayStatus represents the observed state of a Gateway.
type GatewayStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 GatewayObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Gateway is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,contextforge}
type ContextForgeGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayList contains a list of Gateway
type ContextForgeGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContextForgeGateway `json:"items"`
}

// Gateway type metadata.
var (
	GatewayKind             = reflect.TypeOf(ContextForgeGateway{}).Name()
	GatewayGroupKind        = schema.GroupKind{Group: Group, Kind: GatewayKind}.String()
	GatewayKindAPIVersion   = GatewayKind + "." + SchemeGroupVersion.String()
	GatewayGroupVersionKind = SchemeGroupVersion.WithKind(GatewayKind)
)

func init() {
	SchemeBuilder.Register(&ContextForgeGateway{}, &ContextForgeGatewayList{})
}
