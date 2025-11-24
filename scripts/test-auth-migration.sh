#!/bin/bash
# Test script to verify session-based authentication migration
# Ensures no references to old Basic Auth variables remain

set -e

echo "========================================"
echo "Authentication Migration Test Suite"
echo "========================================"
echo ""

FAILED=0

# Test 1: Check for authUsername/authPassword references
echo "Test 1: Checking for obsolete auth variables..."
if grep -rn "authUsername\|authPassword" web/ --include="*.js" --include="*.html" >/dev/null 2>&1; then
    echo "❌ FAILED: Found references to authUsername or authPassword:"
    grep -rn "authUsername\|authPassword" web/ --include="*.js" --include="*.html"
    FAILED=1
else
    echo "✅ PASSED: No references to authUsername or authPassword"
fi
echo ""

# Test 2: Verify session management files exist
echo "Test 2: Checking session management files..."
FILES=(
    "internal/auth/session.go"
    "internal/api/auth_handlers.go"
    "web/login.html"
    "web/login.js"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ Found: $file"
    else
        echo "❌ FAILED: Missing file: $file"
        FAILED=1
    fi
done
echo ""

# Test 3: Verify session functions exist
echo "Test 3: Checking session functions..."
FUNCTIONS=(
    "InitSessionStore"
    "SessionMiddleware"
    "CreateSession"
    "DestroySession"
    "GetSession"
)

for func in "${FUNCTIONS[@]}"; do
    if grep -q "func $func" internal/auth/session.go; then
        echo "✅ Found function: $func"
    else
        echo "❌ FAILED: Missing function: $func"
        FAILED=1
    fi
done
echo ""

# Test 4: Verify login/logout endpoints
echo "Test 4: Checking login/logout endpoints..."
if grep -q "handleLogin\|handleLogout" internal/api/auth_handlers.go; then
    echo "✅ Found: Login/logout handlers"
else
    echo "❌ FAILED: Missing login/logout handlers"
    FAILED=1
fi
echo ""

# Test 5: Check for improper Basic Auth in vulnerability functions
echo "Test 5: Checking vulnerability functions for improper auth..."
VULN_FUNCS=(
    "preloadVulnerabilityScans"
    "scanAllImages"
    "rescanImage"
    "updateTrivyDB"
    "viewVulnerabilityDetails"
    "openVulnerabilitySettingsModal"
)

for func in "${VULN_FUNCS[@]}"; do
    # Get the function body
    if grep -A 30 "function $func" web/app.js | grep -q "btoa(authUsername"; then
        echo "❌ FAILED: $func still uses Basic Auth with authUsername"
        FAILED=1
    else
        echo "✅ $func: No improper Basic Auth"
    fi
done
echo ""

# Test 6: Verify logout button exists
echo "Test 6: Checking logout button in UI..."
if grep -q "logout()" web/index.html; then
    echo "✅ Found: Logout button"
else
    echo "❌ FAILED: Missing logout button"
    FAILED=1
fi
echo ""

# Test 7: Verify SESSION_SECRET in documentation
echo "Test 7: Checking SESSION_SECRET in documentation..."
if grep -q "SESSION_SECRET" README.md; then
    echo "✅ Found: SESSION_SECRET documented"
else
    echo "❌ FAILED: SESSION_SECRET not documented"
    FAILED=1
fi
echo ""

# Test 8: Check fetchWithAuth function exists (for 401 handling)
echo "Test 8: Checking 401 redirect handling..."
if grep -q "function fetchWithAuth\|async function fetchWithAuth" web/app.js; then
    echo "✅ Found: fetchWithAuth function with 401 handling"
else
    echo "⚠️  WARNING: fetchWithAuth function not found (may use different pattern)"
fi
echo ""

# Summary
echo "========================================"
if [ $FAILED -eq 0 ]; then
    echo "✅ ALL TESTS PASSED"
    echo "========================================"
    exit 0
else
    echo "❌ SOME TESTS FAILED"
    echo "========================================"
    exit 1
fi
