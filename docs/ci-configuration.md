# CI/CD Configuration

This document describes the required tokens, secrets, and variables for the GitHub/Forgejo workflows.

## Secrets

### RENOVATE_TOKEN

Used by the Renovate workflow to create pull requests for dependency updates.

#### GitHub (Personal Access Token)

**Fine-grained PAT (recommended):**

| Permission     | Access         | Purpose                              |
|----------------|----------------|--------------------------------------|
| Contents       | Read and Write | Read/write repository files          |
| Issues         | Read and Write | Create/update issues                 |
| Pull requests  | Read and Write | Create/update pull requests          |
| Workflows      | Read and Write | Update `.github/workflows/` files    |
| Metadata       | Read           | Basic repository metadata (automatic)|

**Classic PAT:**

| Scope        | Purpose                                |
|--------------|----------------------------------------|
| `repo`       | Full control of private repositories   |
| `workflow`   | Update workflow files                  |

For public repositories only, `public_repo` is sufficient instead of full `repo` scope.

#### Forgejo/Gitea

Create token under **Settings > Applications > Generate New Token** with:

| Scope               | Purpose                           |
|---------------------|-----------------------------------|
| `write:repository`  | Read / Write repository contents  |
| `write:issue`       | Read / Write issues               |
| `read:user`         | Read user information (optional)  |
| `read:organization` | Access organization informations  |

The token must have permission to create branches and pull requests.

**Note:** The workflow uses `RENOVATE_TOKEN || GITHUB_TOKEN` as fallback. The default `GITHUB_TOKEN` has limitations (cannot trigger workflows in other repos, limited permissions). A dedicated `RENOVATE_TOKEN` is recommended for full functionality.

### GITHUB_TOKEN

Automatically provided by GitHub/Forgejo Actions. Used by most workflows for:

- Checking out code
- Pushing Docker images to container registry
- Creating releases
- Publishing Helm charts

No manual configuration required.

### CODECOV_TOKEN

Optional. Used by the CI workflow to upload test coverage reports to Codecov. Only used when running on GitHub (not Forgejo).

## Variables

### DOCKER_HOST

Used by the CI workflow's Docker Build job to connect to the Docker daemon.

| Platform | Behavior                                                                 |
|----------|--------------------------------------------------------------------------|
| GitHub   | Not required (uses default Unix socket `/var/run/docker.sock`)           |
| Forgejo  | Required when using Docker-in-Docker (DinD) setup                        |

**Configuration for Forgejo with Docker-in-Docker:**

Set this repository variable under **Settings > Actions > Variables**:

```
DOCKER_HOST=tcp://docker-in-docker:2375
```

The value should match the hostname and port of your DinD container as configured in your runner setup.

### CONTAINER_REGISTRY

Used by the release workflow to specify the container registry for Docker images.

| Platform | Behavior                                                                 |
|----------|--------------------------------------------------------------------------|
| GitHub   | Automatically uses `ghcr.io` (GitHub Container Registry)                 |
| Forgejo  | Uses the value of `CONTAINER_REGISTRY` variable (without protocol prefix)|

**Configuration for Forgejo:**

Set this repository variable under **Settings > Actions > Variables**:

```
CONTAINER_REGISTRY=forge.example.com
```

The value should be the registry hostname without protocol prefix (no `https://`).

**Examples:**

| Registry Type        | Value                      |
|----------------------|----------------------------|
| Forgejo built-in     | `forge.example.com`     |
| Docker Hub           | `docker.io`                |
| Custom registry      | `registry.example.com`     |

The workflow automatically detects the platform and selects the appropriate registry:

```yaml
REGISTRY: ${{ github.server_url == 'https://github.com' && 'ghcr.io' || format('{0}', vars.CONTAINER_REGISTRY) }}
```

## Workflow Overview

| Workflow       | Triggers                          | Required Secrets/Variables                    |
|----------------|-----------------------------------|-----------------------------------------------|
| CI             | Push to main, PRs, manual         | CODECOV_TOKEN (optional), DOCKER_HOST (Forgejo) |
| Release        | Tags `v*`, manual                 | GITHUB_TOKEN, CONTAINER_REGISTRY              |
| Helm Release   | Tags `v*`, manual                 | GITHUB_TOKEN                                  |
| Renovate       | Schedule, push to main, manual    | RENOVATE_TOKEN                                |
