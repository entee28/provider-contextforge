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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1alpha1 "github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
	"github.com/entee28/provider-contextforge/internal/clients/contextforge"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

var errBoom = errors.New("boom")

// fake implements the service interface via function fields, so each test
// case only wires up the calls it actually expects; an unset field panics on
// use, which is the point — it flags a call you didn't expect (e.g. Create
// calling CreateTeam when it should have adopted via the pre-check instead).
type fake struct {
	MockGetTeam           func(ctx context.Context, id string) (*contextforge.Team, error)
	MockCreateTeam        func(ctx context.Context, t contextforge.TeamCreate) (*contextforge.Team, error)
	MockUpdateTeam        func(ctx context.Context, id string, t contextforge.TeamUpdate) (*contextforge.Team, error)
	MockDeleteTeam        func(ctx context.Context, id string) error
	MockResolveTeamBySlug func(ctx context.Context, slug string) (string, error)
}

func (f *fake) GetTeam(ctx context.Context, id string) (*contextforge.Team, error) {
	return f.MockGetTeam(ctx, id)
}
func (f *fake) CreateTeam(ctx context.Context, t contextforge.TeamCreate) (*contextforge.Team, error) {
	return f.MockCreateTeam(ctx, t)
}
func (f *fake) UpdateTeam(ctx context.Context, id string, t contextforge.TeamUpdate) (*contextforge.Team, error) {
	return f.MockUpdateTeam(ctx, id, t)
}
func (f *fake) DeleteTeam(ctx context.Context, id string) error {
	return f.MockDeleteTeam(ctx, id)
}
func (f *fake) ResolveTeamBySlug(ctx context.Context, slug string) (string, error) {
	return f.MockResolveTeamBySlug(ctx, slug)
}

func testCR() *v1alpha1.ContextForgeTeam {
	v := &v1alpha1.ContextForgeTeam{}
	v.SetName("test-team")
	v.SetNamespace("test-ns")
	v.Spec.ForProvider = v1alpha1.TeamParameters{
		Name: "test-team",
	}
	return v
}

func withExternalName(cr *v1alpha1.ContextForgeTeam, name string) *v1alpha1.ContextForgeTeam {
	meta.SetExternalName(cr, name)
	return cr
}

func TestObserve(t *testing.T) {
	type fields struct{ service service }
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeTeam
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
			fields: fields{service: &fake{}},
			args:   args{ctx: context.Background(), cr: testCR()},
			want:   want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"NotFound": {
			reason: "Observe should report ResourceExists false when GetTeam 404s.",
			fields: fields{service: &fake{
				MockGetTeam: func(_ context.Context, _ string) (*contextforge.Team, error) {
					return nil, &contextforge.APIError{StatusCode: 404}
				},
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: false}},
		},
		"GetTeamError": {
			reason: "Observe should wrap and return any GetTeam error that isn't a 404.",
			fields: fields{service: &fake{
				MockGetTeam: func(_ context.Context, _ string) (*contextforge.Team, error) { return nil, errBoom },
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{o: managed.ExternalObservation{}, err: errors.Wrap(errBoom, errGetTeam)},
		},
		"UpToDate": {
			reason: "Observe should report ResourceUpToDate true when the CF team matches spec.",
			fields: fields{service: &fake{
				MockGetTeam: func(_ context.Context, id string) (*contextforge.Team, error) {
					return &contextforge.Team{ID: id, Name: "test-team"}, nil
				},
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}},
		},
		"NotUpToDate": {
			reason: "Observe should report ResourceUpToDate false when the CF team's name has drifted from spec.",
			fields: fields{service: &fake{
				MockGetTeam: func(_ context.Context, id string) (*contextforge.Team, error) {
					return &contextforge.Team{ID: id, Name: "renamed-elsewhere"}, nil
				},
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{o: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
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
	type fields struct{ service service }
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeTeam
	}
	type want struct {
		c            managed.ExternalCreation
		err          error
		externalName string // "" means SetExternalName was never reached
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ResolveTeamBySlugError": {
			reason: "Create should wrap and return a real ResolveTeamBySlug error without ever calling CreateTeam.",
			fields: fields{service: &fake{
				MockResolveTeamBySlug: func(_ context.Context, _ string) (string, error) { return "", errBoom },
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errResolveTeamBySlug)},
		},
		"AdoptExisting": {
			reason: "Create should adopt a team found by the slug pre-check instead of calling CreateTeam.",
			fields: fields{service: &fake{
				MockResolveTeamBySlug: func(_ context.Context, _ string) (string, error) { return "team-uuid", nil },
				MockGetTeam: func(_ context.Context, id string) (*contextforge.Team, error) {
					return &contextforge.Team{ID: id, Name: "test-team"}, nil
				},
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "team-uuid"},
		},
		"AdoptExistingGetTeamError": {
			reason: "Create should wrap a GetTeam error hit while adopting, and must not set the external-name.",
			fields: fields{service: &fake{
				MockResolveTeamBySlug: func(_ context.Context, _ string) (string, error) { return "team-uuid", nil },
				MockGetTeam: func(_ context.Context, _ string) (*contextforge.Team, error) {
					return nil, errBoom
				},
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errGetTeam)},
		},
		"CreateNew": {
			reason: "Create should call CreateTeam and set the external-name when no existing team matches the slug.",
			fields: fields{service: &fake{
				MockResolveTeamBySlug: func(_ context.Context, _ string) (string, error) { return "", contextforge.ErrTeamNotFound },
				MockCreateTeam: func(_ context.Context, tc contextforge.TeamCreate) (*contextforge.Team, error) {
					return &contextforge.Team{ID: "new-team-uuid", Name: tc.Name}, nil
				},
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, externalName: "new-team-uuid"},
		},
		"CreateNewError": {
			reason: "Create should wrap and return any CreateTeam error, and must not set the external-name.",
			fields: fields{service: &fake{
				MockResolveTeamBySlug: func(_ context.Context, _ string) (string, error) { return "", contextforge.ErrTeamNotFound },
				MockCreateTeam: func(_ context.Context, _ contextforge.TeamCreate) (*contextforge.Team, error) {
					return nil, errBoom
				},
			}},
			args: args{ctx: context.Background(), cr: testCR()},
			want: want{c: managed.ExternalCreation{}, err: errors.Wrap(errBoom, errCreateTeam)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
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
	type fields struct{ service service }
	type args struct {
		ctx context.Context
		cr  *v1alpha1.ContextForgeTeam
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
			reason: "Update should succeed when UpdateTeam succeeds.",
			fields: fields{service: &fake{
				MockUpdateTeam: func(_ context.Context, id string, _ contextforge.TeamUpdate) (*contextforge.Team, error) {
					return &contextforge.Team{ID: id, Name: "test-team"}, nil
				},
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{u: managed.ExternalUpdate{}},
		},
		"UpdateTeamError": {
			reason: "Update should wrap and return any UpdateTeam error.",
			fields: fields{service: &fake{
				MockUpdateTeam: func(_ context.Context, _ string, _ contextforge.TeamUpdate) (*contextforge.Team, error) {
					return nil, errBoom
				},
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{u: managed.ExternalUpdate{}, err: errors.Wrap(errBoom, errUpdateTeam)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{service: tc.fields.service}
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
		cr  *v1alpha1.ContextForgeTeam
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
			reason: "Delete should succeed when DeleteTeam succeeds.",
			fields: fields{service: &fake{
				MockDeleteTeam: func(_ context.Context, _ string) error { return nil },
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{},
		},
		"DeleteTeamError": {
			reason: "Delete should wrap and return any DeleteTeam error.",
			fields: fields{service: &fake{
				MockDeleteTeam: func(_ context.Context, _ string) error { return errBoom },
			}},
			args: args{ctx: context.Background(), cr: withExternalName(testCR(), "team-uuid")},
			want: want{d: managed.ExternalDelete{}, err: errors.Wrap(errBoom, errDeleteTeam)},
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
