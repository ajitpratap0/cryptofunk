# Branch Protection Configuration

This document outlines the recommended branch protection rules for the CryptoFunk repository.

## Main Branch Protection

Configure the following settings for the `main` branch via GitHub Settings → Branches → Branch protection rules:

### Required Status Checks
✅ **Require status checks to pass before merging**
- Enable "Require branches to be up to date before merging"
- Required checks:
  - `lint`
  - `test`
  - `build`
  - `integration-test`
  - `security`

### Pull Request Requirements
✅ **Require pull request reviews before merging**
- Required approvals: 1
- Dismiss stale pull request approvals when new commits are pushed
- Require review from Code Owners (if CODEOWNERS file exists)

### Additional Settings
✅ **Require conversation resolution before merging**
- Ensures all review comments are addressed

✅ **Require linear history**
- Prevents merge commits, maintains clean git history

✅ **Do not allow bypassing the above settings**
- Administrators included (recommended for production)

❌ **Allow force pushes** - DISABLED
❌ **Allow deletions** - DISABLED

## Develop Branch Protection

Configure the following settings for the `develop` branch:

### Required Status Checks
✅ **Require status checks to pass before merging**
- Required checks:
  - `lint`
  - `test`
  - `build`

### Pull Request Requirements
✅ **Require pull request reviews before merging**
- Required approvals: 1

## Configuring via GitHub CLI

You can also configure branch protection using the GitHub CLI:

```bash
# Install gh CLI: https://cli.github.com/

# Authenticate
gh auth login

# Configure main branch protection
gh api repos/ajitpratap0/cryptofunk/branches/main/protection \
  --method PUT \
  --field required_status_checks='{"strict":true,"contexts":["lint","test","build","integration-test","security"]}' \
  --field required_pull_request_reviews='{"dismissal_restrictions":{},"dismiss_stale_reviews":true,"require_code_owner_reviews":false,"required_approving_review_count":1}' \
  --field required_conversation_resolution='{"enabled":true}' \
  --field required_linear_history='{"enabled":true}' \
  --field allow_force_pushes='{"enabled":false}' \
  --field allow_deletions='{"enabled":false}' \
  --field enforce_admins='{"enabled":true}'
```

## Codecov Integration

The CI workflow uploads coverage reports to Codecov. To enable coverage checks:

1. Sign up for Codecov: https://about.codecov.io/
2. Add the repository
3. Add `codecov/patch` and `codecov/project` to required status checks
4. Configure coverage thresholds in `.codecov.yml` (optional)

## Status Badge

Add to README.md:

```markdown
![CI Status](https://github.com/ajitpratap0/cryptofunk/workflows/CI/badge.svg)
[![codecov](https://codecov.io/gh/ajitpratap0/cryptofunk/branch/main/graph/badge.svg)](https://codecov.io/gh/ajitpratap0/cryptofunk)
```

## Verification

After configuring branch protection:

1. Create a test PR with failing tests
2. Verify merge is blocked
3. Fix tests and verify merge is allowed
4. Check that status checks appear correctly

## Troubleshooting

### Status checks not appearing
- Ensure the workflow has run at least once
- Check workflow names match exactly in branch protection settings
- Verify workflows are triggered on `pull_request` events

### Cannot merge despite passing checks
- Check if branch is up to date with base branch
- Verify all required checks are green (not just some)
- Check for unresolved conversations

### Admin bypass not working
- Verify "Include administrators" is not enabled in branch protection
- Check repository settings for admin privileges
