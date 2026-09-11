package authresolve

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
)

type Result struct {
	AuthType                  *string
	Username, Password, Token *string
	Headers                   []map[string]string
	QPKey, QPValue            *string
	OAuthCfg                  map[string]any
}

func Resolve(ctx context.Context, kube client.Client, ns string, a *v1alpha1.AuthConfig) (auth Result, err error) {
	if a == nil {
		return Result{}, nil
	}

	t := string(a.Type)

	switch a.Type {
	case "basic":
		return resolveBasicAuth(ctx, kube, a.Basic.SecretName, ns, t)
	case "bearer":
		return resolveBearerAuth(ctx, kube, a.Bearer.SecretRef, ns, t)
	case "headers":
		return resolveHeadersAuth(ctx, kube, a.Headers.Headers, ns, t)
	case "queryParam":
		return resolveQueryParamAuth(ctx, kube, *a.QueryParam, ns, t)
	case "oauth":
		return resolveOAuthAuth(ctx, kube, *a.OAuth, ns, t)
	}
	return Result{}, nil
}

func getKey(ctx context.Context, kube client.Client, ref xpv2.LocalSecretKeySelector, namespace string) (string, error) {
	s := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}, s); err != nil {
		return "", err
	}
	v, ok := s.Data[ref.Key]
	if !ok {
		return "", errors.Errorf("key %q not found in secret %q", ref.Key, ref.Name)
	}
	return string(v), nil
}

func resolveBasicAuth(ctx context.Context, kube client.Client, secretName string, namespace string, authType string) (Result, error) {
	s := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: namespace,
	}, s); err != nil {
		return Result{}, err
	}
	u, ok := s.Data["username"]
	if !ok {
		return Result{}, errors.Errorf("key \"username\" not found in secret %q", secretName)
	}
	p, ok := s.Data["password"]
	if !ok {
		return Result{}, errors.Errorf("key \"password\" not found in secret %q", secretName)
	}
	us, ps := string(u), string(p)
	return Result{
		AuthType: &authType,
		Username: &us,
		Password: &ps,
	}, nil
}

func resolveBearerAuth(ctx context.Context, kube client.Client, secretRef xpv2.LocalSecretKeySelector, namespace string, authType string) (Result, error) {
	tok, err := getKey(ctx, kube, secretRef, namespace)
	if err != nil {
		return Result{}, err
	}
	return Result{
		AuthType: &authType,
		Token:    &tok,
	}, nil
}

func resolveHeadersAuth(ctx context.Context, kube client.Client, headers []v1alpha1.HeaderEntry, namespace string, authType string) (Result, error) {
	entries := make([]map[string]string, 0, len(headers))
	for _, h := range headers {
		v, err := getKey(ctx, kube, h.SecretRef, namespace)
		if err != nil {
			return Result{}, err
		}
		entries = append(entries, map[string]string{h.Name: v})
	}
	return Result{
		AuthType: &authType,
		Headers:  entries,
	}, nil
}

func resolveQueryParamAuth(ctx context.Context, kube client.Client, queryParam v1alpha1.QueryParamAuth, namespace string, authType string) (Result, error) {
	qpValue, err := getKey(ctx, kube, queryParam.SecretRef, namespace)
	if err != nil {
		return Result{}, err
	}
	return Result{
		AuthType: &authType,
		QPKey:    &queryParam.Name,
		QPValue:  &qpValue,
	}, nil
}

func resolveOAuthAuth(ctx context.Context, kube client.Client, oauth v1alpha1.OAuthAuth, namespace string, authType string) (Result, error) {
	cfg := map[string]any{
		"grant_type": oauth.GrantType,
		"client_id":  oauth.ClientID,
	}
	if oauth.ClientSecretRef != nil {
		clientSecret, err := getKey(ctx, kube, *oauth.ClientSecretRef, namespace)
		if err != nil {
			return Result{}, err
		}
		cfg["client_secret"] = clientSecret
	}

	if oauth.TokenURL != "" {
		cfg["token_url"] = oauth.TokenURL
	}
	if oauth.AuthorizationURL != "" {
		cfg["authorization_url"] = oauth.AuthorizationURL
	}
	if oauth.Issuer != "" {
		cfg["issuer"] = oauth.Issuer
	}
	if oauth.JWKSURI != "" {
		cfg["jwks_uri"] = oauth.JWKSURI
	}
	if oauth.RedirectURI != "" {
		cfg["redirect_uri"] = oauth.RedirectURI
	}
	if len(oauth.Scopes) > 0 {
		cfg["scopes"] = oauth.Scopes
	}

	return Result{
		AuthType: &authType,
		OAuthCfg: cfg,
	}, nil
}
