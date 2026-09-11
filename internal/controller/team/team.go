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

package team

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"

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
	"github.com/entee28/provider-contextforge/internal/controller/providerconfig"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"

	errNewClient         = "cannot create new Service"
	errGetTeam           = "cannot get Team"
	errCreateTeam        = "cannot create Team"
	errUpdateTeam        = "cannot update Team"
	errDeleteTeam        = "cannot delete Team"
	errResolveTeamBySlug = "cannot resolve team ID by slug"
)

func toObservation(t *contextforge.Team) v1alpha1.TeamObservation {
	return v1alpha1.TeamObservation{
		ID:          t.ID,
		Slug:        t.Slug,
		Name:        t.Name,
		MaxMembers:  t.MaxMembers,
		MemberCount: t.MemberCount,
		IsActive:    &t.IsActive,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func toTeamCreate(cr *v1alpha1.ContextForgeTeam) contextforge.TeamCreate {
	p := cr.Spec.ForProvider
	return contextforge.TeamCreate{
		Name:        p.Name,
		Description: p.Description,
		Visibility:  contextforge.TeamVisibilityPrivate,
		MaxMembers:  p.MaxMembers,
	}
}

func isUpToDate(cr *v1alpha1.ContextForgeTeam, t *contextforge.Team) bool {
	p := cr.Spec.ForProvider
	if p.Name != t.Name {
		return false
	}
	if p.Description != nil && *p.Description != t.Description {
		return false
	}
	return intPtrEqual(p.MaxMembers, t.MaxMembers)
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// SetupGated adds a controller that reconciles Team managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Team controller"))
		}
	}, v1alpha1.TeamGroupVersionKind)
	return nil
}

func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.TeamGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.ContextForgeTeam](&connector{
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ContextForgeTeamList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.ContextForgeTeamList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.TeamGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.ContextForgeTeam{}).
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
func (c *connector) Connect(ctx context.Context, cr *v1alpha1.ContextForgeTeam) (managed.TypedExternalClient[*v1alpha1.ContextForgeTeam], error) {
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

	return &external{service: svc}, nil
}

type service interface {
	GetTeam(ctx context.Context, id string) (*contextforge.Team, error)
	CreateTeam(ctx context.Context, t contextforge.TeamCreate) (*contextforge.Team, error)
	UpdateTeam(ctx context.Context, id string, t contextforge.TeamUpdate) (*contextforge.Team, error)
	DeleteTeam(ctx context.Context, id string) error
	ResolveTeamBySlug(ctx context.Context, slug string) (string, error)
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service service
}

func (c *external) Observe(ctx context.Context, cr *v1alpha1.ContextForgeTeam) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	if id == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	team, err := c.service.GetTeam(ctx, id)
	if contextforge.IsNotFound(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetTeam)
	}

	cr.Status.AtProvider = toObservation(team)
	cr.Status.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: isUpToDate(cr, team),
	}, nil
}

func (c *external) Create(ctx context.Context, cr *v1alpha1.ContextForgeTeam) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv2.Creating())

	name := cr.Spec.ForProvider.Name

	id, err := c.service.ResolveTeamBySlug(ctx, name)
	if err != nil && !errors.Is(err, contextforge.ErrTeamNotFound) {
		return managed.ExternalCreation{}, errors.Wrap(err, errResolveTeamBySlug)
	}
	if err == nil {
		t, err := c.service.GetTeam(ctx, id)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errGetTeam)
		}
		meta.SetExternalName(cr, id)
		cr.Status.AtProvider = toObservation(t)
		return managed.ExternalCreation{}, nil
	}

	t, err := c.service.CreateTeam(ctx, toTeamCreate(cr))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateTeam)
	}

	meta.SetExternalName(cr, t.ID)
	cr.Status.AtProvider = toObservation(t)

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, cr *v1alpha1.ContextForgeTeam) (managed.ExternalUpdate, error) {
	id := meta.GetExternalName(cr)
	p := cr.Spec.ForProvider

	t, err := c.service.UpdateTeam(ctx, id, contextforge.TeamUpdate{
		Description: p.Description,
		MaxMembers:  p.MaxMembers,
	})
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateTeam)
	}

	cr.Status.AtProvider = toObservation(t)

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, cr *v1alpha1.ContextForgeTeam) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv2.Deleting())
	id := meta.GetExternalName(cr)

	if err := c.service.DeleteTeam(ctx, id); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteTeam)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
