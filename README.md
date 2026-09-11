# provider-contextforge

`provider-contextforge` is a [Crossplane](https://crossplane.io/) Provider for
[ContextForge (IBM MCP Gateway)](https://github.com/IBM/mcp-context-forge). It
lets you manage ContextForge Teams, Gateways, Virtual Servers, and A2A Agents
as Kubernetes Custom Resources.

## Managed resource types

All resources below live in the `mcp.contextforge.entee28.io/v1alpha1` API
group and are namespaced — a resource's Kubernetes namespace corresponds to
its ContextForge Team.

| Kind                        | Description                                                                    |
|------------------------------|--------------------------------------------------------------------------------|
| `ContextForgeTeam`          | A ContextForge Team, the top-level scoping/ownership unit for other resources. |
| `ContextForgeGateway`       | A federated upstream MCP server registered with ContextForge.                  |
| `ContextForgeVirtualServer` | A virtual MCP server composed of tools/resources/prompts/A2A agents.           |
| `ContextForgeA2AAgent`      | An Agent-to-Agent (A2A) protocol agent registered with ContextForge.           |

Provider connection configuration (`ProviderConfig` / `ClusterProviderConfig`)
lives in the `contextforge.entee28.io/v1alpha1` group.

## Prerequisites

- A running [Crossplane](https://crossplane.io/) v2 control plane.
- A reachable ContextForge (MCP Gateway) instance and an API credential for it
  (e.g. a bearer token).

## Configuring a ProviderConfig

Point the provider at your ContextForge instance and give it a credential
Secret containing an API token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: cf-test
  name: contextforge-creds
type: Opaque
stringData:
  credentials: <your-contextforge-api-token>
---
apiVersion: contextforge.entee28.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
  namespace: cf-test
spec:
  baseURL: https://contextforge.example.com
  credentials:
    source: Secret
    secretRef:
      namespace: cf-test
      name: contextforge-creds
      key: credentials
```

`ClusterProviderConfig` works the same way for a cluster-scoped (non-namespaced)
credential.

## Example resources

```yaml
apiVersion: mcp.contextforge.entee28.io/v1alpha1
kind: ContextForgeTeam
metadata:
  name: platform-team
  namespace: cf-test
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
  forProvider:
    name: platform-team
---
apiVersion: mcp.contextforge.entee28.io/v1alpha1
kind: ContextForgeGateway
metadata:
  name: internal-mcp-gateway
  namespace: cf-test
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
  forProvider:
    url: https://internal-mcp-server.example.com
    transport: STREAMABLEHTTP
    visibility: team
    teamRef:
      name: platform-team
---
apiVersion: mcp.contextforge.entee28.io/v1alpha1
kind: ContextForgeVirtualServer
metadata:
  name: platform-virtual-server
  namespace: cf-test
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
  forProvider:
    visibility: team
    teamRef:
      name: platform-team
---
apiVersion: mcp.contextforge.entee28.io/v1alpha1
kind: ContextForgeA2AAgent
metadata:
  name: research-agent
  namespace: cf-test
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
  forProvider:
    endpointUrl: https://research-agent.example.com
    visibility: team
    teamRef:
      name: platform-team
```

`ContextForgeGateway` and `ContextForgeA2AAgent` both support an optional
`auth` block (`basic`, `bearer`, `headers`, `queryParam`, or `oauth`) that
resolves credentials from a Kubernetes Secret rather than storing them inline
— see `apis/mcp/v1alpha1/auth_types.go` for the full shape.

## Developing

1. Run `make submodules` to initialize the "build" Make submodule used for CI/CD.
1. Run `make generate` to regenerate CRDs / deepcopy code after changing types
   under `apis/`.
1. Run `make reviewable` to run code generation, linters, and tests.
1. Run `make build` to build the provider.

New resource types are added under `apis/mcp/v1alpha1/` and
`internal/controller/<resource>/`, and registered in
`internal/controller/register.go`'s `SetupGated`. The ContextForge HTTP client
lives in `internal/clients/contextforge/`.

Refer to Crossplane's [CONTRIBUTING.md] file for more information on how the
Crossplane community prefers to work. The [Provider Development][provider-dev]
guide may also be of use.

[CONTRIBUTING.md]: https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md
[provider-dev]: https://github.com/crossplane/crossplane/blob/master/contributing/guide-provider-development.md
