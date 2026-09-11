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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1alpha1 "github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
	"github.com/entee28/provider-contextforge/internal/clients/contextforge"
)

var errBoom = errors.New("boom")

const (
	testGatewayURL = "https://mcp.example.com"
	testTransport  = "http"
	testTeamID     = "team-id"
)

type mockGatewayService struct {
	MockGetGateway    func(ctx context.Context, idOrNameOrSlug string) (*contextforge.Gateway, error)
	MockCreateGateway func(ctx context.Context, t contextforge.GatewayCreate) (*contextforge.Gateway, error)
	MockUpdateGateway func(ctx context.Context, id string, t contextforge.GatewayUpdate) (*contextforge.Gateway, error)
	MockDeleteGateway func(ctx context.Context, id string) error
}

func (m *mockGatewayService) GetGateway(ctx context.Context, idOrNameOrSlug string) (*contextforge.Gateway, error) {
	return m.MockGetGateway(ctx, idOrNameOrSlug)
}
func (m *mockGatewayService) CreateGateway(ctx context.Context, t contextforge.GatewayCreate) (*contextforge.Gateway, error) {
	return m.MockCreateGateway(ctx, t)
}
func (m *mockGatewayService) UpdateGateway(ctx context.Context, id string, t contextforge.GatewayUpdate) (*contextforge.Gateway, error) {
	return m.MockUpdateGateway(ctx, id, t)
}
func (m *mockGatewayService) DeleteGateway(ctx context.Context, id string) error {
	return m.MockDeleteGateway(ctx, id)
}

func testCR() *v1alpha1.ContextForgeGateway {
	v := &v1alpha1.ContextForgeGateway{}
	v.SetName("test-gateway")
	v.SetNamespace("test-ns")
	v.Spec.ForProvider = v1alpha1.GatewayParameters{
		URL:        testGatewayURL,
		Transport:  testTransport,
		TeamRef:    v1alpha1.TeamReference{Name: "test-team"},
		Visibility: "public",
	}
	return v
}

func withExternalName(cr *v1alpha1.ContextForgeGateway, id string) *v1alpha1.ContextForgeGateway {
	meta.SetExternalName(cr, id)
	return cr
}

func withDeletionTimestamp(cr *v1alpha1.ContextForgeGateway) *v1alpha1.ContextForgeGateway {
	now := metav1.Now()
	cr.SetDeletionTimestamp(&now)
	cr.SetFinalizers([]string{"finalizer.managedresource.crossplane.io"})
	return cr
}

func fakeKubeWithTeam(teamID string) client.Client {
	team := &v1alpha1.ContextForgeTeam{
		ObjectMeta: metav1.ObjectMeta{Name: "test-team", Namespace: "test-ns"},
	}
	team.Status.AtProvider.ID = teamID
	s := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(team).Build()
}

func emptyKubeClient() client.Client {
	s := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return fake.NewClientBuilder().WithScheme(s).Build()
}

func TestObserve(t *testing.T) {
	type fields struct {
		service service
		kube    client.Client
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeGateway
	}
	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"NoExternalName": {
			reason: "Observe should report ResourceExists false without calling the service when no external-name is set yet.",
			fields: fields{service: &mockGatewayService{}, kube: fakeKubeWithTeam(testTeamID)},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"NotFound": {
			reason: "Observe should report ResourceExists false when GetGateway returns 404.",
			fields: fields{
				service: &mockGatewayService{
					MockGetGateway: func(_ context.Context, _ string) (*contextforge.Gateway, error) {
						return nil, &contextforge.APIError{StatusCode: 404}
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"GetGatewayError": {
			reason: "Observe should wrap and return any GetGateway error that isn't a 404.",
			fields: fields{
				service: &mockGatewayService{
					MockGetGateway: func(_ context.Context, _ string) (*contextforge.Gateway, error) { return nil, errBoom },
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{o: managed.ExternalObservation{}, err: errors.Wrap(errBoom, errGetGateway)},
		},
		"DeletedResourceSkipsTeamResolution": {
			reason: "Observe should report ResourceExists true for a resource pending deletion without resolving the (possibly already-deleted) Team, so Delete can still be called and the finalizer removed.",
			fields: fields{
				service: &mockGatewayService{
					MockGetGateway: func(_ context.Context, id string) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: id, Name: "test-gateway", TeamID: testTeamID, URL: testGatewayURL, Transport: testTransport, Visibility: visibility}, nil
					},
				},
				kube: emptyKubeClient(),
			},
			args: args{ctx: context.Background(), cr: withDeletionTimestamp(withExternalName(testCR(), "gw-uuid"))},
			want: want{o: managed.ExternalObservation{ResourceExists: true}},
		},
		"UpToDate": {
			reason: "Observe should report ResourceUpToDate true when the CF gateway matches spec.",
			fields: fields{
				service: &mockGatewayService{
					MockGetGateway: func(_ context.Context, id string) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: id, Name: "test-gateway", TeamID: testTeamID, URL: testGatewayURL, Transport: testTransport, Visibility: visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}},
		},
		"NotUpToDate": {
			reason: "Observe should report ResourceUpToDate false when the CF gateway has drifted from spec.",
			fields: fields{
				service: &mockGatewayService{
					MockGetGateway: func(_ context.Context, id string) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: id, Name: "renamed-elsewhere", TeamID: testTeamID, URL: testGatewayURL, Transport: testTransport, Visibility: visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{service: tc.fields.service, kube: tc.fields.kube}
			got, err := e.Observe(tc.args.ctx, tc.args.cr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type fields struct {
		service service
		kube    client.Client
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeGateway
	}
	type want struct {
		c            managed.ExternalCreation
		err          error
		externalName string
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"CreateSuccess": {
			reason: "Create should call CreateGateway and set the external-name.",
			fields: fields{
				service: &mockGatewayService{
					MockCreateGateway: func(_ context.Context, gc contextforge.GatewayCreate) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: "gw-new-uuid", Name: gc.Name, TeamID: gc.TeamID, URL: gc.URL, Transport: gc.Transport, Visibility: visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "gw-new-uuid"},
		},
		"AdoptOn409": {
			reason: "Create should adopt a gateway found via conflict on 409.",
			fields: fields{
				service: &mockGatewayService{
					MockCreateGateway: func(_ context.Context, _ contextforge.GatewayCreate) (*contextforge.Gateway, error) {
						return nil, &contextforge.APIError{StatusCode: 409}
					},
					MockGetGateway: func(_ context.Context, name string) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: "gw-adopted-uuid", Name: name, TeamID: testTeamID, URL: testGatewayURL, Transport: testTransport, Visibility: visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "gw-adopted-uuid"},
		},
		"CreateError": {
			reason: "Create should wrap and return any CreateGateway error.",
			fields: fields{
				service: &mockGatewayService{
					MockCreateGateway: func(_ context.Context, _ contextforge.GatewayCreate) (*contextforge.Gateway, error) {
						return nil, errBoom
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errCreateGateway)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{service: tc.fields.service, kube: tc.fields.kube}
			got, err := e.Create(tc.args.ctx, tc.args.cr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.c, got); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want, +got:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.externalName, meta.GetExternalName(tc.args.cr)); diff != "" {
				t.Errorf("\n%s\ne.Create(...): -want external-name, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type fields struct {
		service service
		kube    client.Client
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeGateway
	}
	type want struct {
		u   managed.ExternalUpdate
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"UpdateSuccess": {
			reason: "Update should succeed when UpdateGateway succeeds.",
			fields: fields{
				service: &mockGatewayService{
					MockUpdateGateway: func(_ context.Context, id string, _ contextforge.GatewayUpdate) (*contextforge.Gateway, error) {
						visibility := contextforge.Visibility("public")
						return &contextforge.Gateway{ID: id, Name: "test-gateway", TeamID: testTeamID, URL: testGatewayURL, Transport: testTransport, Visibility: visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{u: managed.ExternalUpdate{}},
		},
		"UpdateGatewayError": {
			reason: "Update should wrap and return any UpdateGateway error.",
			fields: fields{
				service: &mockGatewayService{
					MockUpdateGateway: func(_ context.Context, _ string, _ contextforge.GatewayUpdate) (*contextforge.Gateway, error) {
						return nil, errBoom
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{u: managed.ExternalUpdate{}, err: errors.Wrap(errBoom, errUpdateGateway)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{service: tc.fields.service, kube: tc.fields.kube}
			got, err := e.Update(tc.args.ctx, tc.args.cr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Update(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.u, got); diff != "" {
				t.Errorf("\n%s\ne.Update(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type fields struct {
		service service
		kube    client.Client
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeGateway
	}
	type want struct {
		d   managed.ExternalDelete
		err error
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"DeleteSuccess": {
			reason: "Delete should succeed when DeleteGateway succeeds.",
			fields: fields{
				service: &mockGatewayService{
					MockDeleteGateway: func(_ context.Context, _ string) error { return nil },
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{},
		},
		"DeleteGatewayError": {
			reason: "Delete should wrap and return any DeleteGateway error.",
			fields: fields{
				service: &mockGatewayService{
					MockDeleteGateway: func(_ context.Context, _ string) error { return errBoom },
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "gw-uuid")},
			want: want{d: managed.ExternalDelete{}, err: errors.Wrap(errBoom, errDeleteGateway)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{service: tc.fields.service, kube: tc.fields.kube}
			got, err := e.Delete(tc.args.ctx, tc.args.cr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Delete(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.d, got); diff != "" {
				t.Errorf("\n%s\ne.Delete(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
