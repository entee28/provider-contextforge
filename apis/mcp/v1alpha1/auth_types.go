package v1alpha1

import xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

// +kubebuilder:validation:Enum=basic;bearer;headers;queryParam;oauth
type AuthType string

// +kubebuilder:validation:XValidation:rule="self.type != 'basic' || has(self.basic)", message="basic is required when type is basic"
// +kubebuilder:validation:XValidation:rule="self.type != 'bearer' || has(self.bearer)", message="bearer is required when type is bearer"
// +kubebuilder:validation:XValidation:rule="self.type != 'headers' || has(self.headers)", message="headers is required when type is headers"
// +kubebuilder:validation:XValidation:rule="self.type != 'queryParam' || has(self.queryParam)", message="queryParam is required when type is queryParam"
// +kubebuilder:validation:XValidation:rule="self.type != 'oauth' || has(self.oauth)", message="oauth is required when type is oauth"
type AuthConfig struct {
	Type AuthType `json:"type"`

	// +optional
	Basic      *BasicAuth      `json:"basic,omitempty"`
	Bearer     *BearerAuth     `json:"bearer,omitempty"`
	Headers    *HeadersAuth    `json:"headers,omitempty"`
	QueryParam *QueryParamAuth `json:"queryParam,omitempty"`
	OAuth      *OAuthAuth      `json:"oauth,omitempty"`
}

type BasicAuth struct {
	// Secret must contain both a "username" and "password" key
	SecretName string `json:"secretName"`
}

type BearerAuth struct {
	SecretRef xpv2.LocalSecretKeySelector `json:"secretRef"`
}

type HeadersAuth struct {
	Headers []HeaderEntry `json:"headers"`
}
type HeaderEntry struct {
	Name      string                      `json:"name"`
	SecretRef xpv2.LocalSecretKeySelector `json:"secretRef"`
}

type QueryParamAuth struct {
	Name      string                      `json:"name"`
	SecretRef xpv2.LocalSecretKeySelector `json:"secretRef"`
}

type OAuthAuth struct {
	GrantType        string                       `json:"grantType"`
	ClientID         string                       `json:"clientId"`
	ClientSecretRef  *xpv2.LocalSecretKeySelector `json:"clientSecretRef,omitempty"`
	TokenURL         string                       `json:"tokenUrl,omitempty"`
	AuthorizationURL string                       `json:"authorizationUrl,omitempty"`
	Issuer           string                       `json:"issuer,omitempty"`
	JWKSURI          string                       `json:"jwksUri,omitempty"`
	RedirectURI      string                       `json:"redirectUri,omitempty"`
	Scopes           []string                     `json:"scopes,omitempty"`
}
