# ContextForge Provider Examples

This directory contains example manifests for using provider-contextforge to manage ContextForge resources.

## Prerequisites

- A running Crossplane v2 control plane
- provider-contextforge installed in your cluster
- A running ContextForge (MCP Gateway) instance with an API token

## Quick Start

### 1. Set up the ProviderConfig

First, create a namespace and configure access to your ContextForge instance:

```bash
kubectl apply -f provider/config.yaml
```

This creates:
- A `contextforge` namespace
- A Secret containing your ContextForge API token
- A `ProviderConfig` resource pointing to your ContextForge instance

**Edit `provider/config.yaml` before applying:**
- Replace `your-api-token-here` with your actual ContextForge API token
- Replace `https://contextforge.example.com` with your ContextForge base URL

### 2. Create a Team

Teams are the top-level organizational unit. Create one with:

```bash
kubectl apply -f sample/team.yaml
```

Verify the team was created:

```bash
kubectl get contextforgeteam -n contextforge
kubectl describe contextforgeteam example-team -n contextforge
```

### 3. Create a Gateway

Gateways register upstream MCP servers with ContextForge:

```bash
kubectl apply -f sample/gateway.yaml
```

**Edit `sample/gateway.yaml` before applying:**
- Set `url` to your actual MCP server endpoint
- Set `teamRef.name` to match your team name
- Configure `auth` if the upstream MCP server requires authentication

Verify:

```bash
kubectl get contextforgegateway -n contextforge
```

### 4. Create a Virtual Server

Virtual Servers compose tools/resources/prompts/agents into a single MCP endpoint:

```bash
kubectl apply -f sample/virtualserver.yaml
```

**Edit `sample/virtualserver.yaml` before applying:**
- Set `teamRef.name` to match your team name
- Add actual tool, resource, prompt, and agent IDs if available

Verify:

```bash
kubectl get contextforgevirtualserver -n contextforge
```

### 5. Create an A2A Agent

A2A (Agent-to-Agent) Agents register custom agents:

```bash
kubectl apply -f sample/a2aagent.yaml
```

**Edit `sample/a2aagent.yaml` before applying:**
- Set `endpointUrl` to your agent's endpoint
- Set `teamRef.name` to match your team name
- Configure `auth` if your agent requires authentication

Verify:

```bash
kubectl get contextforgea2aagent -n contextforge
```

## Cleaning Up

Remove all resources:

```bash
kubectl delete -f sample/
kubectl delete -f provider/config.yaml
```

## Troubleshooting

### Resources stuck in "Creating" or "Updating"

Check the controller logs:

```bash
kubectl logs -n crossplane-system -l app=provider-contextforge
```

Check the resource status:

```bash
kubectl describe contextforgegateway example-gateway -n contextforge
```

### Authentication errors

Verify your ProviderConfig credentials are correct:

```bash
kubectl get providerconfig -n contextforge
kubectl describe providerconfig default -n contextforge
```

Make sure the Secret containing your API token exists and has the correct key:

```bash
kubectl get secret contextforge-credentials -n contextforge
```

### Team reference not found

Ensure the Team exists before creating resources that reference it:

```bash
kubectl get contextforgeteam -n contextforge
```

## Resource Types

| Kind | Group | Namespace | Description |
|---|---|---|---|
| `ContextForgeTeam` | `mcp.contextforge.entee28.io` | Namespaced | A team for organizing resources |
| `ContextForgeGateway` | `mcp.contextforge.entee28.io` | Namespaced | An upstream MCP server |
| `ContextForgeVirtualServer` | `mcp.contextforge.entee28.io` | Namespaced | A composed MCP server |
| `ContextForgeA2AAgent` | `mcp.contextforge.entee28.io` | Namespaced | An A2A protocol agent |
| `ProviderConfig` | `contextforge.entee28.io` | Namespaced | Connection config for ContextForge |
| `ClusterProviderConfig` | `contextforge.entee28.io` | Cluster | Cluster-wide connection config |

## Next Steps

- Read the full provider documentation in the main [README.md](../README.md)
- Check the CRD definitions in `package/crds/` for all available fields
- Explore the [ContextForge documentation](https://github.com/IBM/mcp-context-forge) for API details
