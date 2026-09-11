package providerconfig

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	apisv1alpha1 "github.com/entee28/provider-contextforge/apis/v1alpha1"
)

const (
	errGetPC    = "cannot get ProviderConfig"
	errGetCPC   = "cannot get ClusterProviderConfig"
	errGetCreds = "cannot get credentials"
)

func Resolve(ctx context.Context, kube client.Client, ref *xpv2.ProviderConfigReference, namespace string) (baseURL string, creds []byte, err error) {
	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, pc); err != nil {
			return "", nil, errors.Wrap(err, errGetPC)
		}
		data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, kube, pc.Spec.Credentials.CommonCredentialSelectors)
		return pc.Spec.BaseURL, data, errors.Wrap(err, errGetCreds)
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return "", nil, errors.Wrap(err, errGetCPC)
		}
		data, err := resource.CommonCredentialExtractor(ctx, cpc.Spec.Credentials.Source, kube, cpc.Spec.Credentials.CommonCredentialSelectors)
		return cpc.Spec.BaseURL, data, errors.Wrap(err, errGetCreds)

	default:
		return "", nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}
}
