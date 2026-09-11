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

package virtualserver

import (
	"context"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/google/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
	apisv1alpha1 "github.com/entee28/provider-contextforge/apis/v1alpha1"
	"github.com/entee28/provider-contextforge/internal/clients/contextforge"
	"github.com/entee28/provider-contextforge/internal/controller/providerconfig"
	"github.com/entee28/provider-contextforge/internal/controller/sliceutil"
	"github.com/entee28/provider-contextforge/internal/controller/teamresolve"
)

const (
	errTrackPCUsage = "cannot track ProviderConfig usage"

	errNewClient    = "cannot create new Service"
	errGetServer    = "cannot get Virtual Server"
	errCreateServer = "cannot create Virtual Server"
	errUpdateServer = "cannot update Virtual Server"
	errDeleteServer = "cannot delete Virtual Server"
)

const (
	connectionDetailSSE     = "sse"
	connectionDetailMessage = "message"
)

// SetupGated adds a controller that reconciles VirtualServer managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup VirtualServer controller"))
		}
	}, v1alpha1.VirtualServerGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles VirtualServer managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.VirtualServerGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*v1alpha1.ContextForgeVirtualServer](&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: contextforge.NewClient,
		}),
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.ContextForgeVirtualServerList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.ContextForgeVirtualServerList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.VirtualServerGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.ContextForgeVirtualServer{}).
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
func (c *connector) Connect(ctx context.Context, cr *v1alpha1.ContextForgeVirtualServer) (managed.TypedExternalClient[*v1alpha1.ContextForgeVirtualServer], error) {
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
	GetServer(ctx context.Context, id string) (*contextforge.Server, error)
	CreateServer(ctx context.Context, teamID string, v contextforge.Visibility, s contextforge.ServerCreate) (*contextforge.Server, error)
	UpdateServer(ctx context.Context, id string, u contextforge.ServerUpdate) (*contextforge.Server, error)
	DeleteServer(ctx context.Context, id string) error
	BaseURL() string
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube client.Client
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service service
}

var idNamespace = uuid.MustParse("2f7c9e41-6b3a-4d82-a5f9-1c7e6b0d3a24")

func deterministicID(cr *v1alpha1.ContextForgeVirtualServer) string {
	u := uuid.NewSHA1(idNamespace, []byte(cr.GetNamespace()+"/"+cr.GetName()))
	return strings.ReplaceAll(u.String(), "-", "")
}

func toObservation(s *contextforge.Server) v1alpha1.VirtualServerObservation {
	return v1alpha1.VirtualServerObservation{
		ID:         s.ID,
		Name:       s.Name,
		TeamID:     s.TeamID,
		OwnerEmail: s.OwnerEmail,
		Visibility: string(s.Visibility),
		Enabled:    &s.Enabled,
		Tags:       contextforge.TagLabels(s.Tags),
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func isUpToDate(cr *v1alpha1.ContextForgeVirtualServer, resolvedTeamID string, s *contextforge.Server) bool {
	return scalarFieldsUpToDate(cr, resolvedTeamID, s) && associationsUpToDate(cr.Spec.ForProvider, s)
}

func scalarFieldsUpToDate(cr *v1alpha1.ContextForgeVirtualServer, resolvedTeamID string, s *contextforge.Server) bool {
	p := cr.Spec.ForProvider
	if resolvedName(cr) != s.Name {
		return false
	}
	if resolvedTeamID != s.TeamID {
		return false
	}
	if p.Visibility != string(s.Visibility) {
		return false
	}
	if p.Description != nil && *p.Description != s.Description {
		return false
	}
	return true
}

func associationsUpToDate(p v1alpha1.VirtualServerParameters, s *contextforge.Server) bool {
	return sliceutil.EqualUnordered(p.Tags, contextforge.TagLabels(s.Tags)) &&
		sliceutil.EqualUnordered(p.AssociatedTools, s.AssociatedToolIDs) &&
		sliceutil.EqualUnordered(p.AssociatedResources, s.AssociatedResources) &&
		sliceutil.EqualUnordered(p.AssociatedPrompts, s.AssociatedPrompts) &&
		sliceutil.EqualUnordered(p.AssociatedA2AAgents, s.AssociatedA2AAgents)
}

func connectionDetails(baseURL, id string) managed.ConnectionDetails {
	return managed.ConnectionDetails{
		connectionDetailSSE:     []byte(baseURL + "/v1/servers/" + id + "/sse"),
		connectionDetailMessage: []byte(baseURL + "/v1/servers/" + id + "/message"),
	}
}

func (c *external) Observe(ctx context.Context, cr *v1alpha1.ContextForgeVirtualServer) (managed.ExternalObservation, error) {
	id := meta.GetExternalName(cr)
	// Simulate external resource doesn't exist, and enter to create the flow
	if id == "" {
		id = deterministicID(cr)
	}

	srv, err := c.service.GetServer(ctx, id)
	if contextforge.IsNotFound(err) {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetServer)
	}

	cr.Status.AtProvider = toObservation(srv)
	// Now the resource is in sync and ready to use, so mark it as available.
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
		ResourceUpToDate: isUpToDate(cr, teamId, srv),

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: connectionDetails(c.service.BaseURL(), id),
	}, nil
}

func toServerCreate(cr *v1alpha1.ContextForgeVirtualServer, id string) contextforge.ServerCreate {
	p := cr.Spec.ForProvider

	return contextforge.ServerCreate{
		ID:                  id,
		Name:                resolvedName(cr),
		Description:         p.Description,
		Icon:                p.Icon,
		Tags:                p.Tags,
		AssociatedTools:     nonNil(p.AssociatedTools),
		AssociatedResources: nonNil(p.AssociatedResources),
		AssociatedPrompts:   nonNil(p.AssociatedPrompts),
		AssociatedA2AAgents: nonNil(p.AssociatedA2AAgents),
	}
}

func toServerUpdate(cr *v1alpha1.ContextForgeVirtualServer, teamID string) contextforge.ServerUpdate {
	p := cr.Spec.ForProvider
	name := resolvedName(cr)
	visibility := contextforge.Visibility(p.Visibility)

	return contextforge.ServerUpdate{
		Name:                &name,
		Description:         p.Description,
		Icon:                p.Icon,
		TeamID:              &teamID,
		Visibility:          &visibility,
		Tags:                nonNil(p.Tags),
		AssociatedTools:     nonNil(p.AssociatedTools),
		AssociatedResources: nonNil(p.AssociatedResources),
		AssociatedPrompts:   nonNil(p.AssociatedPrompts),
		AssociatedA2AAgents: nonNil(p.AssociatedA2AAgents),
	}
}

func resolvedName(cr *v1alpha1.ContextForgeVirtualServer) string {
	if cr.Spec.ForProvider.Name != nil {
		return *cr.Spec.ForProvider.Name
	}
	return cr.GetName()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (c *external) Create(ctx context.Context, cr *v1alpha1.ContextForgeVirtualServer) (managed.ExternalCreation, error) {
	cr.Status.SetConditions(xpv2.Creating())

	id := deterministicID(cr)

	teamId, err := teamresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.TeamRef.Name)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	visibility := contextforge.Visibility(cr.Spec.ForProvider.Visibility)
	srv, err := c.service.CreateServer(ctx, teamId, visibility, toServerCreate(cr, id))
	if contextforge.IsConflict(err) {
		srv, err = c.service.GetServer(ctx, id)
	}
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateServer)
	}

	cr.Status.AtProvider = toObservation(srv)
	meta.SetExternalName(cr, id)

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: connectionDetails(c.service.BaseURL(), srv.ID),
	}, nil
}

func (c *external) Update(ctx context.Context, cr *v1alpha1.ContextForgeVirtualServer) (managed.ExternalUpdate, error) {
	id := deterministicID(cr)

	teamId, err := teamresolve.Resolve(ctx, c.kube, cr.GetNamespace(), cr.Spec.ForProvider.TeamRef.Name)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	srv, err := c.service.UpdateServer(ctx, id, toServerUpdate(cr, teamId))

	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateServer)
	}

	cr.Status.AtProvider = toObservation(srv)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: connectionDetails(c.service.BaseURL(), id),
	}, nil
}

func (c *external) Delete(ctx context.Context, cr *v1alpha1.ContextForgeVirtualServer) (managed.ExternalDelete, error) {
	cr.Status.SetConditions(xpv2.Deleting())
	id := deterministicID(cr)

	err := c.service.DeleteServer(ctx, id)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteServer)
	}

	return managed.ExternalDelete{}, nil
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}
