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

package gateway

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

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
	"github.com/entee28/provider-contextforge/internal/controller/providerconfig"
	"github.com/entee28/provider-contextforge/internal/controller/sliceutil"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"

	errNewClient     = "cannot create new Service"
	errGetGateway    = "cannot get Gateway"
	errCreateGateway = "cannot create Gateway"
	errUpdateGateway = "cannot update Gateway"
	errDeleteGateway = "cannot delete Gateway"
	errGetTeamRef    = "cannot get Team reference"
	errResolveAuth   = "cannot resolve auth"
	errTeamNotReady  = "Referenced Team is not ready"
)

func resolvedName(cr *v1alpha1.ContextForgeGateway) string {
	if cr.Spec.ForProvider.Name != nil {
		return *cr.Spec.ForProvider.Name
	}
	return cr.GetName()
}

func (c *external) resolveTeamID(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (string, error) {
	team := &v1alpha1.ContextForgeTeam{}
	key := types.NamespacedName{
		Name:      cr.Spec.ForProvider.TeamRef.Name,
		Namespace: cr.GetNamespace(),
	}
	if err := c.kube.Get(ctx, key, team); err != nil {
		return "", errors.Wrap(err, errGetTeamRef)
	}
	if team.Status.AtProvider.ID == "" {
		return "", errors.New(errTeamNotReady)
	}

	return team.Status.AtProvider.ID, nil
}

func toObservation(g *contextforge.Gateway) v1alpha1.GatewayObservation {
	return v1alpha1.GatewayObservation{
		ID:                   g.ID,
		Name:                 g.Name,
		Slug:                 g.Slug,
		URL:                  g.URL,
		Transport:            g.Transport,
		TeamID:               g.TeamID,
		Visibility:           string(g.Visibility),
		Tags:                 g.Tags,
		GatewayMode:          g.GatewayMode,
		Enabled:              g.Enabled,
		Reachable:            g.Reachable,
		Status:               g.Status,
		StatusMessage:        g.StatusMessage,
		LastError:            g.LastError,
		RegistrationAttempts: g.RegistrationAttempts,
		NextRetryAt:          g.NextRetryAt,
		LastSeen:             g.LastSeen,
		ToolCount:            g.ToolCount,
		CreatedAt:            g.CreatedAt,
		UpdatedAt:            g.UpdatedAt,
	}
}

func toGatewayCreate(cr *v1alpha1.ContextForgeGateway, teamID string, auth authresolve.Result) contextforge.GatewayCreate {
	p := cr.Spec.ForProvider
	return contextforge.GatewayCreate{
		Name:                resolvedName(cr),
		URL:                 p.URL,
		Description:         p.Description,
		Transport:           p.Transport,
		TeamID:              teamID,
		Visibility:          contextforge.Visibility(p.Visibility),
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

func toGatewayUpdate(cr *v1alpha1.ContextForgeGateway, teamID string, auth authresolve.Result) contextforge.GatewayUpdate {
	p := cr.Spec.ForProvider
	name := resolvedName(cr)
	visibility := contextforge.Visibility(p.Visibility)

	return contextforge.GatewayUpdate{
		Name:                &name,
		URL:                 &p.URL,
		Description:         p.Description,
		Transport:           &p.Transport,
		TeamID:              &teamID,
		Visibility:          &visibility,
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

func isUpToDate(cr *v1alpha1.ContextForgeGateway, resolvedTeamID string, g *contextforge.Gateway) bool {
	p := cr.Spec.ForProvider

	if resolvedName(cr) != g.Name {
		return false
	}
	if resolvedTeamID != g.TeamID {
		return false
	}
	if p.Description != nil && *p.Description != g.Description {
		return false
	}
	if p.Transport != g.Transport {
		return false
	}
	if p.Visibility != string(g.Visibility) {
		return false
	}
	if !sliceutil.EqualUnordered(p.Tags, g.Tags) {
		return false
	}

	return true
}

// SetupGated adds a controller that reconciles Gateway managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Gateway controller"))
		}
	}, v1alpha1.GatewayGroupVersionKind)
	return nil
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.GatewayGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.ContextForgeGateway](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ContextForgeGatewayList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.GatewayList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.GatewayGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.ContextForgeGateway{}).
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
func (c *connector) Connect(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (managed.TypedExternalClient[*v1alpha1.ContextForgeGateway], error) {
	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	ref := cr.GetProviderConfigReference()

	baseURL, data, err := providerconfig.Resolve(ctx, c.kube, ref, cr.GetNamespace())

	if err != nil {
		return nil, err
	}

	svc, err := c.newServiceFn(baseURL, data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{kube: c.kube, service: svc}, nil
}

type service interface {
	GetGateway(ctx context.Context, idOrNameOrSlug string) (*contextforge.Gateway, error)
	CreateGateway(ctx context.Context, t contextforge.GatewayCreate) (*contextforge.Gateway, error)
	UpdateGateway(ctx context.Context, id string, t contextforge.GatewayUpdate) (*contextforge.Gateway, error)
	DeleteGateway(ctx context.Context, id string) error
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube client.Client
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service service
}

func (c *external) Observe(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	g, err := c.service.GetGateway(ctx, id)
	if contextforge.IsNotFound(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetGateway)
	}

	cr.Status.AtProvider = toObservation(g)
	cr.Status.SetConditions(xpv2.Available())

	if meta.WasDeleted(cr) {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}

	teamId, err := c.resolveTeamID(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(cr, teamId, g),
	}, nil
}

func (c *external) Create(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv2.Creating())

	teamId, err := c.resolveTeamID(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	auth, err := authresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.Auth)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errResolveAuth)
	}

	g, err := c.service.CreateGateway(ctx, toGatewayCreate(cr, teamId, auth))
	if contextforge.IsConflict(err) {
		g, err = c.service.GetGateway(ctx, resolvedName(cr))
	}
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateGateway)
	}

	meta.SetExternalName(cr, g.ID)
	cr.Status.AtProvider = toObservation(g)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (managed.ExternalUpdate, error) {
	id := meta.GetExternalName(cr)

	teamId, err := c.resolveTeamID(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	auth, err := authresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.Auth)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errResolveAuth)
	}

	g, err := c.service.UpdateGateway(ctx, id, toGatewayUpdate(cr, teamId, auth))
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateGateway)
	}

	cr.Status.AtProvider = toObservation(g)

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, cr *v1alpha1.ContextForgeGateway) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv2.Deleting())

	id := meta.GetExternalName(cr)

	err := c.service.DeleteGateway(ctx, id)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteGateway)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
