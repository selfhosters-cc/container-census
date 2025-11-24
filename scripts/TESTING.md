# Testing Guide

## Authentication Migration Tests

This directory contains automated tests to prevent authentication-related regressions after migrating from HTTP Basic Auth to session-based authentication.

## Test Scripts

### 1. Full Test Suite: `test-auth-migration.sh`

Comprehensive test suite that validates:
- ✅ No obsolete `authUsername` or `authPassword` references
- ✅ Session management files exist
- ✅ Session functions are implemented
- ✅ Login/logout endpoints exist
- ✅ Vulnerability functions use proper authentication
- ✅ Logout button exists in UI
- ✅ Documentation includes SESSION_SECRET
- ✅ 401 redirect handling is implemented

**Run the tests:**
```bash
./scripts/test-auth-migration.sh
```

**Exit codes:**
- `0` = All tests passed
- `1` = One or more tests failed

**Use in CI/CD:**
```yaml
# Example GitHub Actions workflow
- name: Run authentication tests
  run: ./scripts/test-auth-migration.sh
```

### 2. Pre-Commit Hook: `pre-commit-hook.sh`

Prevents commits that contain obsolete authentication variables.

**Install the hook:**
```bash
ln -s ../../scripts/pre-commit-hook.sh .git/hooks/pre-commit
```

**What it does:**
- Checks staged `.js` and `.html` files for `authUsername` or `authPassword`
- Blocks the commit if found
- Provides helpful error message with correct pattern

**Test the hook:**
```bash
# Create a file with bad pattern
echo "const x = authUsername;" > web/test.js
git add web/test.js
git commit -m "test"  # This will be blocked

# Clean up
git reset web/test.js
rm web/test.js
```

## Why These Tests Matter

After migrating to session-based authentication, we removed the `authUsername` and `authPassword` global variables. However, some functions still referenced them, causing runtime errors like:

```
Failed to load vulnerability details: authUsername is not defined
```

These tests ensure:
1. No code references the removed variables
2. All API calls use session cookies (sent automatically)
3. The migration is complete and won't regress

## Session-Based Auth Pattern

**✅ Correct (session cookies sent automatically):**
```javascript
const response = await fetch('/api/vulnerabilities/scan/123', {
    method: 'POST'
});
```

**❌ Incorrect (references removed variables):**
```javascript
const response = await fetch('/api/vulnerabilities/scan/123', {
    method: 'POST',
    headers: {
        'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword)
    }
});
```

## Manual Verification

If you need to manually check for regressions:

```bash
# Search for authUsername or authPassword in web files
grep -rn "authUsername\|authPassword" web/ --include="*.js" --include="*.html"

# Should return no results
```

## Troubleshooting

**Tests fail with "command not found":**
- Ensure you're in the repository root: `cd /path/to/container-census`
- Make scripts executable: `chmod +x scripts/*.sh`

**Pre-commit hook not running:**
- Check it's installed: `ls -la .git/hooks/pre-commit`
- Verify symlink: `readlink .git/hooks/pre-commit` should point to `../../scripts/pre-commit-hook.sh`
- Make it executable: `chmod +x scripts/pre-commit-hook.sh`

## Future Improvements

Consider adding:
- [ ] ESLint rule to catch `authUsername`/`authPassword` usage
- [ ] TypeScript validation (if migrating to TS)
- [ ] Integration tests for login flow
- [ ] E2E tests with Playwright/Cypress
