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

package a2aagent

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	v1alpha1 "github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
	apisv1alpha1 "github.com/entee28/provider-contextforge/apis/v1alpha1"
	"github.com/entee28/provider-contextforge/internal/clients/contextforge"
	"github.com/entee28/provider-contextforge/internal/controller/authresolve"
	"github.com/entee28/provider-contextforge/internal/controller/sliceutil"
	"github.com/entee28/provider-contextforge/internal/controller/teamresolve"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCPC       = "cannot get ClusterProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient      = "cannot create new Service"
	errGetA2AAgent    = "cannot get A2A Agent"
	errCreateA2AAgent = "cannot create A2A Agent"
	errUpdateA2AAgent = "cannot update A2A Agent"
	errDeleteA2AAgent = "cannot delete A2A Agent"
	errResolveAuth    = "cannot resolve auth"
)

func resolvedName(cr *v1alpha1.ContextForgeA2AAgent) string {
	if cr.Spec.ForProvider.Name != nil {
		return *cr.Spec.ForProvider.Name
	}
	return cr.GetName()
}

// toRawExtension marshals a map into an apiextensionsv1.JSON for CRD storage.
// Returns nil if m is empty, so an absent/empty map round-trips as an absent field.
func toRawExtension(m map[string]any) *apiextensionsv1.JSON {
	if len(m) == 0 {
		return nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return &apiextensionsv1.JSON{Raw: raw}
}

// fromRawExtension unmarshals an apiextensionsv1.JSON back into a map for
// sending to the ContextForge API. Returns nil if j is nil/empty.
func fromRawExtension(j *apiextensionsv1.JSON) map[string]any {
	if j == nil || len(j.Raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(j.Raw, &m); err != nil {
		return nil
	}
	return m
}

// jsonMapsEqual compares two arbitrary JSON-like maps by their marshaled
// form. encoding/json sorts map keys, so this is stable regardless of
// insertion order, and a nil/empty map on either side marshals identically.
func jsonMapsEqual(a, b map[string]any) bool {
	ab, aerr := json.Marshal(a)
	bb, berr := json.Marshal(b)
	if aerr != nil || berr != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

func toObservation(a *contextforge.A2AAgent) v1alpha1.A2AAgentObservation {
	return v1alpha1.A2AAgentObservation{
		ID:           a.ID,
		Name:         a.Name,
		Slug:         a.Slug,
		EndpointURL:  a.EndpointURL,
		TeamID:       a.TeamID,
		Visibility:   string(a.Visibility),
		Tags:         contextforge.TagLabels(a.Tags),
		Enabled:      a.Enabled,
		Reachable:    a.Reachable,
		Capabilities: toRawExtension(a.Capabilities),
		Config:       toRawExtension(a.Config),
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

func toA2AAgentCreate(cr *v1alpha1.ContextForgeA2AAgent, teamID string, auth authresolve.Result) contextforge.A2AAgentCreate {
	p := cr.Spec.ForProvider
	return contextforge.A2AAgentCreate{
		Name:                resolvedName(cr),
		EndpointURL:         p.EndpointURL,
		Description:         p.Description,
		TeamID:              teamID,
		Visibility:          contextforge.Visibility(p.Visibility),
		AgentType:           p.AgentType,
		ProtocolVersion:     p.ProtocolVersion,
		Capabilities:        fromRawExtension(p.Capabilities),
		Config:              fromRawExtension(p.Config),
		PassthroughHeaders:  p.PassthroughHeaders,
		Tags:                p.Tags,
		AuthType:            auth.AuthType,
		AuthUsername:        auth.Username,
		AuthPassword:        auth.Password,
		AuthToken:           auth.Token,
		AuthHeaders:         auth.Headers,
		AuthQueryParamKey:   auth.QPKey,
		AuthQueryParamValue: auth.QPValue,
		OAuthConfig:         auth.OAuthCfg,
	}
}

func toA2AAgentUpdate(cr *v1alpha1.ContextForgeA2AAgent, teamID string, auth authresolve.Result) contextforge.A2AAgentUpdate {
	p := cr.Spec.ForProvider
	name := resolvedName(cr)
	visibility := contextforge.Visibility(p.Visibility)

	return contextforge.A2AAgentUpdate{
		Name:                &name,
		EndpointURL:         &p.EndpointURL,
		Description:         p.Description,
		TeamID:              &teamID,
		Visibility:          &visibility,
		AgentType:           p.AgentType,
		ProtocolVersion:     p.ProtocolVersion,
		Capabilities:        fromRawExtension(p.Capabilities),
		Config:              fromRawExtension(p.Config),
		PassthroughHeaders:  p.PassthroughHeaders,
		Tags:                p.Tags,
		AuthType:            auth.AuthType,
		AuthUsername:        auth.Username,
		AuthPassword:        auth.Password,
		AuthToken:           auth.Token,
		AuthHeaders:         auth.Headers,
		AuthQueryParamKey:   auth.QPKey,
		AuthQueryParamValue: auth.QPValue,
		OAuthConfig:         auth.OAuthCfg,
	}
}

func isUpToDate(cr *v1alpha1.ContextForgeA2AAgent, resolvedTeamID string, a *contextforge.A2AAgent) bool {
	return scalarFieldsUpToDate(cr, resolvedTeamID, a) && listFieldsUpToDate(cr.Spec.ForProvider, a)
}

func scalarFieldsUpToDate(cr *v1alpha1.ContextForgeA2AAgent, resolvedTeamID string, a *contextforge.A2AAgent) bool {
	p := cr.Spec.ForProvider

	if resolvedName(cr) != a.Name {
		return false
	}
	if resolvedTeamID != a.TeamID {
		return false
	}
	if p.Description != nil && *p.Description != a.Description {
		return false
	}
	if p.Visibility != string(a.Visibility) {
		return false
	}
	if p.AgentType != nil && *p.AgentType != a.AgentType {
		return false
	}
	if p.ProtocolVersion != nil && *p.ProtocolVersion != a.ProtocolVersion {
		return false
	}
	return true
}

func listFieldsUpToDate(p v1alpha1.A2AAgentParameters, a *contextforge.A2AAgent) bool {
	return sliceutil.EqualUnordered(p.Tags, contextforge.TagLabels(a.Tags)) &&
		sliceutil.EqualUnordered(p.PassthroughHeaders, a.PassthroughHeaders) &&
		jsonMapsEqual(fromRawExtension(p.Capabilities), a.Capabilities) &&
		jsonMapsEqual(fromRawExtension(p.Config), a.Config)
}

// SetupGated adds a controller that reconciles A2AAgent managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup A2AAgent controller"))
		}
	}, v1alpha1.A2AAgentGroupVersionKind)
	return nil
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.A2AAgentGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.ContextForgeA2AAgent](&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: contextforge.NewClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // TODO(jbw976) Crossplane needs to update to the new events API, see https://github.com/crossplane/crossplane/issues/7152
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ContextForgeA2AAgentList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.A2AAgentList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.A2AAgentGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.ContextForgeA2AAgent{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        *resource.ProviderConfigUsageTracker
	newServiceFn func(baseURL string, creds []byte) (*contextforge.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, cr *v1alpha1.ContextForgeA2AAgent) (managed.TypedExternalClient[*v1alpha1.ContextForgeA2AAgent], error) {
	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	var cd apisv1alpha1.ProviderCredentials
	var baseURL string

	ref := cr.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetPC)
		}
		cd = pc.Spec.Credentials
		baseURL = pc.Spec.BaseURL
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		cd = cpc.Spec.Credentials
		baseURL = cpc.Spec.BaseURL
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(baseURL, data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{kube: c.kube, service: svc}, nil
}

type service interface {
	GetA2AAgent(ctx context.Context, idOrNameOrSlug string) (*contextforge.A2AAgent, error)
	CreateA2AAgent(ctx context.Context, a contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error)
	UpdateA2AAgent(ctx context.Context, id string, a contextforge.A2AAgentUpdate) (*contextforge.A2AAgent, error)
	DeleteA2AAgent(ctx context.Context, id string) error
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube client.Client
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service service
}

func (c *external) Observe(ctx context.Context, cr *v1alpha1.ContextForgeA2AAgent) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	a, err := c.service.GetA2AAgent(ctx, id)
	if contextforge.IsNotFound(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetA2AAgent)
	}

	cr.Status.AtProvider = toObservation(a)
	cr.Status.SetConditions(xpv2.Available())

	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	teamId, err := teamresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.TeamRef.Name)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: isUpToDate(cr, teamId, a),
	}, nil
}

func (c *external) Create(ctx context.Context, cr *v1alpha1.ContextForgeA2AAgent) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv2.Creating())

	teamId, err := teamresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.TeamRef.Name)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	authConf, err := authresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.Auth)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errResolveAuth)
	}

	a, err := c.service.CreateA2AAgent(ctx, toA2AAgentCreate(cr, teamId, authConf))
	if contextforge.IsConflict(err) {
		a, err = c.service.GetA2AAgent(ctx, resolvedName(cr))
	}
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateA2AAgent)
	}

	meta.SetExternalName(cr, a.ID)
	cr.Status.AtProvider = toObservation(a)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, cr *v1alpha1.ContextForgeA2AAgent) (managed.ExternalUpdate, error) {
	id := meta.GetExternalName(cr)

	teamId, err := teamresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.TeamRef.Name)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	authConf, err := authresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.Auth)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errResolveAuth)
	}

	a, err := c.service.UpdateA2AAgent(ctx, id, toA2AAgentUpdate(cr, teamId, authConf))
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateA2AAgent)
	}

	cr.Status.AtProvider = toObservation(a)

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, cr *v1alpha1.ContextForgeA2AAgent) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv2.Deleting())

	id := meta.GetExternalName(cr)
	if err := c.service.DeleteA2AAgent(ctx, id); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteA2AAgent)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
