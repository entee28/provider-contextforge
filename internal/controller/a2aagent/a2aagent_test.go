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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1alpha1 "github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
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

const (
	testEndpointURL = "https://agent.example.com"
	testTeamID      = "team-id"
	testAgentName   = "test-agent"
)

type mockA2AAgentService struct {
	MockGetA2AAgent    func(ctx context.Context, idOrNameOrSlug string) (*contextforge.A2AAgent, error)
	MockCreateA2AAgent func(ctx context.Context, a contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error)
	MockUpdateA2AAgent func(ctx context.Context, id string, a contextforge.A2AAgentUpdate) (*contextforge.A2AAgent, error)
	MockDeleteA2AAgent func(ctx context.Context, id string) error
}

func (f *mockA2AAgentService) GetA2AAgent(ctx context.Context, idOrNameOrSlug string) (*contextforge.A2AAgent, error) {
	return f.MockGetA2AAgent(ctx, idOrNameOrSlug)
}
func (f *mockA2AAgentService) CreateA2AAgent(ctx context.Context, a contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error) {
	return f.MockCreateA2AAgent(ctx, a)
}
func (f *mockA2AAgentService) UpdateA2AAgent(ctx context.Context, id string, a contextforge.A2AAgentUpdate) (*contextforge.A2AAgent, error) {
	return f.MockUpdateA2AAgent(ctx, id, a)
}
func (f *mockA2AAgentService) DeleteA2AAgent(ctx context.Context, id string) error {
	return f.MockDeleteA2AAgent(ctx, id)
}

func testCR() *v1alpha1.ContextForgeA2AAgent {
	v := &v1alpha1.ContextForgeA2AAgent{}
	v.SetName(testAgentName)
	v.SetNamespace("test-ns")
	v.Spec.ForProvider = v1alpha1.A2AAgentParameters{
		EndpointURL: testEndpointURL,
		TeamRef:     v1alpha1.TeamReference{Name: "test-team"},
		Visibility:  "public",
	}
	return v
}

func withExternalName(cr *v1alpha1.ContextForgeA2AAgent, id string) *v1alpha1.ContextForgeA2AAgent {
	meta.SetExternalName(cr, id)
	return cr
}

func withDeletionTimestamp(cr *v1alpha1.ContextForgeA2AAgent) *v1alpha1.ContextForgeA2AAgent {
	now := metav1.Now()
	cr.SetDeletionTimestamp(&now)
	cr.SetFinalizers([]string{"finalizer.managedresource.crossplane.io"})
	return cr
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := v1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func fakeKubeWithTeam(teamID string) client.Client {
	team := &v1alpha1.ContextForgeTeam{
		ObjectMeta: metav1.ObjectMeta{Name: "test-team", Namespace: "test-ns"},
	}
	team.Status.AtProvider.ID = teamID
	return fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(team).Build()
}

func kubeWithUnreadyTeam() client.Client {
	team := &v1alpha1.ContextForgeTeam{
		ObjectMeta: metav1.ObjectMeta{Name: "test-team", Namespace: "test-ns"},
	}
	return fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(team).Build()
}

func emptyKubeClient() client.Client {
	return fake.NewClientBuilder().WithScheme(newScheme()).Build()
}

// wantTeamRefNotFoundErr computes the exact error resolveTeamID would return
// for a missing Team, by performing the identical Get, rather than guessing
// the fake client's NotFound message string.
func wantTeamRefNotFoundErr(cr *v1alpha1.ContextForgeA2AAgent) error {
	team := &v1alpha1.ContextForgeTeam{}
	key := types.NamespacedName{Name: cr.Spec.ForProvider.TeamRef.Name, Namespace: cr.GetNamespace()}
	err := emptyKubeClient().Get(context.Background(), key, team)
	return errors.Wrap(err, teamresolve.ErrGetTeamRef)
}

func TestObserve(t *testing.T) {
	type fields struct {
		service service
		kube    client.Client
	}
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeA2AAgent
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
			fields: fields{service: &mockA2AAgentService{}, kube: fakeKubeWithTeam(testTeamID)},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"NotFound": {
			reason: "Observe should report ResourceExists false when GetA2AAgent returns 404.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, _ string) (*contextforge.A2AAgent, error) {
						return nil, &contextforge.APIError{StatusCode: 404}
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"GetA2AAgentError": {
			reason: "Observe should wrap and return any GetA2AAgent error that isn't a 404.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, _ string) (*contextforge.A2AAgent, error) { return nil, errBoom },
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{}, err: errors.Wrap(errBoom, errGetA2AAgent)},
		},
		"TeamRefNotFound": {
			reason: "Observe should wrap and return a Get error when the referenced Team doesn't exist.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, id string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: testAgentName}, nil
					},
				},
				kube: emptyKubeClient(),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{}, err: wantTeamRefNotFoundErr(testCR())},
		},
		"TeamNotReady": {
			reason: "Observe should report a not-ready error when the referenced Team has no id in status yet.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, id string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: testAgentName}, nil
					},
				},
				kube: kubeWithUnreadyTeam(),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"DeletedResourceSkipsTeamResolution": {
			reason: "Observe should report ResourceExists true for a resource pending deletion without resolving the (possibly already-deleted) Team, so Delete can still be called and the finalizer removed.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, id string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: testAgentName, TeamID: testTeamID, EndpointURL: testEndpointURL, Visibility: "public"}, nil
					},
				},
				kube: emptyKubeClient(),
			},
			args: args{ctx: context.Background(), cr: withDeletionTimestamp(withExternalName(testCR(), "agent-uuid"))},
			want: want{o: managed.ExternalObservation{ResourceExists: true}},
		},
		"UpToDate": {
			reason: "Observe should report ResourceUpToDate true when the CF agent matches spec.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, id string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: testAgentName, TeamID: testTeamID, EndpointURL: testEndpointURL, Visibility: "public"}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}},
		},
		"NotUpToDate": {
			reason: "Observe should report ResourceUpToDate false when the CF agent has drifted from spec.",
			fields: fields{
				service: &mockA2AAgentService{
					MockGetA2AAgent: func(_ context.Context, id string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: "renamed-elsewhere", TeamID: testTeamID, EndpointURL: testEndpointURL, Visibility: "public"}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.fields.kube, service: tc.fields.service}
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
		cr  *v1alpha1.ContextForgeA2AAgent
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
		"TeamNotReady": {
			reason: "Create should report a not-ready error when the referenced Team has no id in status yet, without calling CreateA2AAgent.",
			fields: fields{service: &mockA2AAgentService{}, kube: kubeWithUnreadyTeam()},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{c: managed.ExternalCreation{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"CreateSuccess": {
			reason: "Create should set the external-name to the server-assigned id on success.",
			fields: fields{
				service: &mockA2AAgentService{
					MockCreateA2AAgent: func(_ context.Context, a contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: "agent-new-uuid", Name: a.Name, TeamID: a.TeamID, EndpointURL: a.EndpointURL, Visibility: a.Visibility}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "agent-new-uuid"},
		},
		"AdoptOn409": {
			reason: "Create should adopt an existing agent found by name when CreateA2AAgent reports a conflict.",
			fields: fields{
				service: &mockA2AAgentService{
					MockCreateA2AAgent: func(_ context.Context, _ contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error) {
						return nil, &contextforge.APIError{StatusCode: 409}
					},
					MockGetA2AAgent: func(_ context.Context, name string) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: "agent-adopted-uuid", Name: name, TeamID: testTeamID}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "agent-adopted-uuid"},
		},
		"CreateA2AAgentError": {
			reason: "Create should wrap and return any CreateA2AAgent error that isn't a conflict.",
			fields: fields{
				service: &mockA2AAgentService{
					MockCreateA2AAgent: func(_ context.Context, _ contextforge.A2AAgentCreate) (*contextforge.A2AAgent, error) {
						return nil, errBoom
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errCreateA2AAgent)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.fields.kube, service: tc.fields.service}
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
		cr  *v1alpha1.ContextForgeA2AAgent
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
		"TeamNotReady": {
			reason: "Update should report a not-ready error when the referenced Team has no id in status yet, without calling UpdateA2AAgent.",
			fields: fields{service: &mockA2AAgentService{}, kube: kubeWithUnreadyTeam()},
			args:   args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want:   want{u: managed.ExternalUpdate{}, err: errors.New(teamresolve.ErrTeamNotReady)},
		},
		"UpdateSuccess": {
			reason: "Update should succeed when UpdateA2AAgent succeeds.",
			fields: fields{
				service: &mockA2AAgentService{
					MockUpdateA2AAgent: func(_ context.Context, id string, u contextforge.A2AAgentUpdate) (*contextforge.A2AAgent, error) {
						return &contextforge.A2AAgent{ID: id, Name: testAgentName, TeamID: *u.TeamID}, nil
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{u: managed.ExternalUpdate{}},
		},
		"UpdateA2AAgentError": {
			reason: "Update should wrap and return any UpdateA2AAgent error, without panicking on the nil result.",
			fields: fields{
				service: &mockA2AAgentService{
					MockUpdateA2AAgent: func(_ context.Context, _ string, _ contextforge.A2AAgentUpdate) (*contextforge.A2AAgent, error) {
						return nil, errBoom
					},
				},
				kube: fakeKubeWithTeam(testTeamID),
			},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{u: managed.ExternalUpdate{}, err: errors.Wrap(errBoom, errUpdateA2AAgent)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{kube: tc.fields.kube, service: tc.fields.service}
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
		cr  *v1alpha1.ContextForgeA2AAgent
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
			reason: "Delete should succeed when DeleteA2AAgent succeeds.",
			fields: fields{service: &mockA2AAgentService{
				MockDeleteA2AAgent: func(_ context.Context, _ string) error { return nil },
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{},
		},
		"DeleteA2AAgentError": {
			reason: "Delete should wrap and return any DeleteA2AAgent error with the correct error constant.",
			fields: fields{service: &mockA2AAgentService{
				MockDeleteA2AAgent: func(_ context.Context, _ string) error { return errBoom },
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "agent-uuid")},
			want: want{d: managed.ExternalDelete{}, err: errors.Wrap(errBoom, errDeleteA2AAgent)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{service: tc.fields.service}
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
