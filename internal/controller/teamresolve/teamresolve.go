package teamresolve

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/entee28/provider-contextforge/apis/mcp/v1alpha1"
)

// Exported so callers (and their tests) can compare against the same
// error messages Resolve wraps, without duplicating the strings.
const (
	ErrGetTeamRef   = "cannot get Team reference"
	ErrTeamNotReady = "Referenced Team is not ready"
)

func Resolve(ctx context.Context, kube client.Client, namespace string, teamRefName string) (string, error) {
	team := &v1alpha1.ContextForgeTeam{}
	key := types.NamespacedName{
		Name:      teamRefName,
		Namespace: namespace,
	}
	if err := kube.Get(ctx, key, team); err != nil {
		return "", errors.Wrap(err, ErrGetTeamRef)
	}
	if team.Status.AtProvider.ID == "" {
		return "", errors.New(ErrTeamNotReady)
	}

	return team.Status.AtProvider.ID, nil
}
