# CI/CD Configuration

This document describes the required tokens and secrets for the GitHub workflows.

## Secrets

### RENOVATE_TOKEN

Used by the Renovate workflow to create pull requests for dependency updates.

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

**Note:** The workflow uses `RENOVATE_TOKEN || GITHUB_TOKEN` as fallback. The default `GITHUB_TOKEN` has limitations (cannot trigger workflows in other repos, limited permissions). A dedicated `RENOVATE_TOKEN` is recommended for full functionality.

### GITHUB_TOKEN

Automatically provided by GitHub Actions. Used by most workflows for:

- Checking out code
- Pushing Docker images to GitHub Container Registry (ghcr.io)
- Creating releases
- Publishing Helm charts

No manual configuration required.

### CODECOV_TOKEN

Optional. Used by the CI workflow to upload test coverage reports to Codecov.

## Workflow Overview

| Workflow       | Triggers                          | Required Secrets                     |
|----------------|-----------------------------------|--------------------------------------|
| CI             | Push to main, PRs, manual         | GITHUB_TOKEN, CODECOV_TOKEN (optional) |
| Release        | Tags `v*`, manual                 | GITHUB_TOKEN                         |
| Helm Release   | Tags `v*`, manual                 | GITHUB_TOKEN                         |
| Cleanup        | Weekly schedule, manual           | GITHUB_TOKEN                         |
| Renovate       | Hourly schedule, push to main, manual | RENOVATE_TOKEN                   |
