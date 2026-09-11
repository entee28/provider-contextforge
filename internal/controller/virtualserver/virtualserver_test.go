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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
	"github.com/entee28/provider-contextforge/internal/clients/contextforge"
	"github.com/entee28/provider-contextforge/internal/controller/teamresolve"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

var errBoom = errors.New("boom")

const testTeamID = "team-uuid"
const testServer = "test-server"

type fake struct {
	MockGetServer    func(ctx context.Context, id string) (*contextforge.Server, error)
	MockCreateServer func(ctx context.Context, teamID string, v contextforge.Visibility, s contextforge.ServerCreate) (*contextforge.Server, error)
	MockUpdateServer func(ctx context.Context, id string, u contextforge.ServerUpdate) (*contextforge.Server, error)
	MockDeleteServer func(ctx context.Context, id string) error
	MockBaseURL      func() string
}

func (f *fake) GetServer(ctx context.Context, id string) (*contextforge.Server, error) {
	return f.MockGetServer(ctx, id)
}
func (f *fake) CreateServer(ctx context.Context, teamID string, v contextforge.Visibility, s contextforge.ServerCreate) (*contextforge.Server, error) {
	return f.MockCreateServer(ctx, teamID, v, s)
}
func (f *fake) UpdateServer(ctx context.Context, id string, u contextforge.ServerUpdate) (*contextforge.Server, error) {
	return f.MockUpdateServer(ctx, id, u)
}
func (f *fake) DeleteServer(ctx context.Context, id string) error {
	return f.MockDeleteServer(ctx, id)
}
func (f *fake) BaseURL() string { return f.MockBaseURL() }

func testCR() *v1alpha1.ContextForgeVirtualServer {
	v := &v1alpha1.ContextForgeVirtualServer{}
	v.SetName(testServer)
	v.SetNamespace("test-ns")
	v.Spec.ForProvider = v1alpha1.VirtualServerParameters{
		TeamRef:    v1alpha1.TeamReference{Name: "default"},
		Visibility: "team",
	}
	return v
}

func withDeletionTimestamp(cr *v1alpha1.ContextForgeVirtualServer) *v1alpha1.ContextForgeVirtualServer {
	now := metav1.Now()
	cr.SetDeletionTimestamp(&now)
	cr.SetFinalizers([]string{"finalizer.managedresource.crossplane.io"})
	return cr
}

// newScheme registers this provider's own types so the fake kube client can
// decode Team/VirtualServer objects.
func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := v1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

// kubeWithTeam seeds a Ready Team (matching testCR's TeamRef) whose
// status.atProvider.id is already resolved.
func kubeWithTeam(id string) client.Client {
	team := &v1alpha1.ContextForgeTeam{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "test-ns"},
		Status:     v1alpha1.TeamStatus{AtProvider: v1alpha1.TeamObservation{ID: id}},
	}
	return fakeclient.NewClientBuilder().WithScheme(newScheme()).WithObjects(team).Build()
}

// kubeWithUnreadyTeam seeds a Team that exists but has no id in status yet —
// simulates Team's own Create not having landed.
func kubeWithUnreadyTeam() client.Client {
	team := &v1alpha1.ContextForgeTeam{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "test-ns"},
	}
	return fakeclient.NewClientBuilder().WithScheme(newScheme()).WithObjects(team).Build()
}

// emptyKubeClient seeds no Team at all — the referenced Team doesn't exist.
func emptyKubeClient() client.Client {
	return fakeclient.NewClientBuilder().WithScheme(newScheme()).Build()
}

// wantTeamRefNotFoundErr computes the exact error resolveTeamID would return
// for a missing Team, by performing the identical Get, rather than guessing
// the fake client's NotFound message string.
func wantTeamRefNotFoundErr(cr *v1alpha1.ContextForgeVirtualServer) error {
	team := &v1alpha1.ContextForgeTeam{}
	key := types.NamespacedName{Name: cr.Spec.ForProvider.TeamRef.Name, Namespace: cr.GetNamespace()}
	err := emptyKubeClient().Get(context.Background(), key, team)
	return errors.Wrap(err, teamresolve.ErrGetTeamRef)
}

func TestObserve(t *testing.T) {
	type fields struct {
		kube    client.Client
		service service
	}

	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeVirtualServer
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
		"NotFound": {
			reason: "Observe should report ResourceExists false when GetServer 404s, before ever resolving the team.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockGetServer: func(ctx context.Context, id string) (*contextforge.Server, error) {
						return nil, &contextforge.APIError{StatusCode: 404}
					},
				},
			},
			args: args{
				ctx: context.Background(),
				cr:  testCR(),
			},
			want: want{
				o: managed.ExternalObservation{ResourceExists: false},
			},
		},
		"GetServerError": {
			reason: "Observe should wrap and return any GetServer error that isn't a 404.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockGetServer: func(_ context.Context, _ string) (*contextforge.Server, error) {
						return nil, errBoom
					},
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{o: managed.ExternalObservation{}, err: errors.Wrap(errBoom, errGetServer)},
		},
		"TeamRefNotFound": {
			reason: "Observe should wrap and return a Get error when the referenced Team doesn't exist.",
			fields: fields{
				kube: emptyKubeClient(),
				service: &fake{
					MockGetServer: func(_ context.Context, id string) (*contextforge.Server, error) {
						return &contextforge.Server{ID: id, Name: testServer}, nil
					},
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{o: managed.ExternalObservation{}, err: wantTeamRefNotFoundErr(testCR())},
		},
		"TeamNotReady": {
			reason: "Observe should report a not-ready error when the referenced Team has no id in status yet.",
			fields: fields{
				kube: kubeWithUnreadyTeam(),
				service: &fake{
					MockGetServer: func(_ context.Context, id string) (*contextforge.Server, error) {
						return &contextforge.Server{ID: id, Name: testServer}, nil
					},
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{o: managed.ExternalObservation{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"DeletedResourceSkipsTeamResolution": {
			reason: "Observe should report ResourceExists true for a resource pending deletion without resolving the (possibly already-deleted) Team, so Delete can still be called and the finalizer removed.",
			fields: fields{
				kube: emptyKubeClient(),
				service: &fake{
					MockGetServer: func(_ context.Context, id string) (*contextforge.Server, error) {
						return &contextforge.Server{ID: id, Name: testServer}, nil
					},
				},
			},
			args: args{ctx: context.Background(), cr: withDeletionTimestamp(testCR())},
			want: want{o: managed.ExternalObservation{ResourceExists: true}},
		},
		"UpToDate": {
			reason: "Observe should report ResourceUpToDate true when the CF server matches spec.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockGetServer: func(_ context.Context, id string) (*contextforge.Server, error) {
						return &contextforge.Server{
							ID:         id,
							Name:       testServer,
							TeamID:     testTeamID,
							Visibility: contextforge.VisibilityTeam,
						}, nil
					},
					MockBaseURL: func() string { return "https://cf.example.com" },
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{o: managed.ExternalObservation{
				ResourceExists:   true,
				ResourceUpToDate: true,
				ConnectionDetails: managed.ConnectionDetails{
					connectionDetailSSE:     []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/sse"),
					connectionDetailMessage: []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/message"),
				},
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{kube: tc.fields.kube, service: tc.fields.service}
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
		kube    client.Client
		service service
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeVirtualServer
	}
	type want struct {
		c            managed.ExternalCreation
		err          error
		externalName string // "" for error paths, where SetExternalName is never reached
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"TeamRefNotFound": {
			reason: "Create should wrap and return a Get error when the referenced Team doesn't exist, without calling CreateServer.",
			fields: fields{kube: emptyKubeClient(), service: &fake{}},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{c: managed.ExternalCreation{}, err: wantTeamRefNotFoundErr(testCR())},
		},
		"TeamNotReady": {
			reason: "Create should report a not-ready error when the referenced Team has no id in status yet.",
			fields: fields{kube: kubeWithUnreadyTeam(), service: &fake{}},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{c: managed.ExternalCreation{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"CreateSuccess": {
			reason: "Create should set the external-name and return connection details on success.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockCreateServer: func(_ context.Context, _ string, _ contextforge.Visibility, s contextforge.ServerCreate) (*contextforge.Server, error) {
						return &contextforge.Server{ID: s.ID, Name: s.Name, TeamID: testTeamID}, nil
					},
					MockBaseURL: func() string { return "https://cf.example.com" },
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{
				c: managed.ExternalCreation{
					ConnectionDetails: managed.ConnectionDetails{
						connectionDetailSSE:     []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/sse"),
						connectionDetailMessage: []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/message"),
					},
				},
				externalName: deterministicID(testCR()),
			},
		},
		"AdoptOn409": {
			reason: "Create should adopt an existing server by GET when CreateServer reports a conflict.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockCreateServer: func(_ context.Context, _ string, _ contextforge.Visibility, _ contextforge.ServerCreate) (*contextforge.Server, error) {
						return nil, &contextforge.APIError{StatusCode: 409}
					},
					MockGetServer: func(_ context.Context, id string) (*contextforge.Server, error) {
						return &contextforge.Server{ID: id, Name: testServer, TeamID: testTeamID}, nil
					},
					MockBaseURL: func() string { return "https://cf.example.com" },
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{
				c: managed.ExternalCreation{
					ConnectionDetails: managed.ConnectionDetails{
						connectionDetailSSE:     []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/sse"),
						connectionDetailMessage: []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/message"),
					},
				},
				externalName: deterministicID(testCR()),
			},
		},
		"CreateServerError": {
			reason: "Create should wrap and return any CreateServer error that isn't a conflict.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockCreateServer: func(_ context.Context, _ string, _ contextforge.Visibility, _ contextforge.ServerCreate) (*contextforge.Server, error) {
						return nil, errBoom
					},
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errCreateServer)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{kube: tc.fields.kube, service: tc.fields.service}
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
		kube    client.Client
		service service
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeVirtualServer
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
		"TeamRefNotFound": {
			reason: "Update should wrap and return a Get error when the referenced Team doesn't exist, without calling UpdateServer.",
			fields: fields{kube: emptyKubeClient(), service: &fake{}},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{u: managed.ExternalUpdate{}, err: wantTeamRefNotFoundErr(testCR())},
		},
		"TeamNotReady": {
			reason: "Update should report a not-ready error when the referenced Team has no id in status yet.",
			fields: fields{kube: kubeWithUnreadyTeam(), service: &fake{}},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{u: managed.ExternalUpdate{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"UpdateSuccess": {
			reason: "Update should return connection details built from the deterministic id.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockUpdateServer: func(_ context.Context, id string, u contextforge.ServerUpdate) (*contextforge.Server, error) {
						return &contextforge.Server{ID: id, Name: testServer, TeamID: *u.TeamID}, nil
					},
					MockBaseURL: func() string { return "https://cf.example.com" },
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{u: managed.ExternalUpdate{
				ConnectionDetails: managed.ConnectionDetails{
					connectionDetailSSE:     []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/sse"),
					connectionDetailMessage: []byte("https://cf.example.com/v1/servers/" + deterministicID(testCR()) + "/message"),
				},
			}},
		},
		"UpdateServerError": {
			reason: "Update should wrap and return any UpdateServer error.",
			fields: fields{
				kube: kubeWithTeam(testTeamID),
				service: &fake{
					MockUpdateServer: func(_ context.Context, _ string, _ contextforge.ServerUpdate) (*contextforge.Server, error) {
						return nil, errBoom
					},
				},
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{u: managed.ExternalUpdate{}, err: errors.Wrap(errBoom, errUpdateServer)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{kube: tc.fields.kube, service: tc.fields.service}
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
	type fields struct{ service service }
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeVirtualServer
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
			reason: "Delete should succeed when DeleteServer succeeds.",
			fields: fields{service: &fake{
				MockDeleteServer: func(_ context.Context, _ string) error { return nil },
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{},
		},
		"DeleteServerError": {
			reason: "Delete should wrap and return any DeleteServer error.",
			fields: fields{service: &fake{
				MockDeleteServer: func(_ context.Context, _ string) error { return errBoom },
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{d: managed.ExternalDelete{}, err: errors.Wrap(errBoom, errDeleteServer)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
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
