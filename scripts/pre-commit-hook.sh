#!/bin/bash
# Git pre-commit hook to prevent authentication regressions
#
# To install: ln -s ../../scripts/pre-commit-hook.sh .git/hooks/pre-commit

# Quick check for authUsername/authPassword in staged JS/HTML files
if git diff --cached --name-only | grep -E '\.(js|html)$' | xargs grep -n "authUsername\|authPassword" 2>/dev/null; then
    echo ""
    echo "❌ COMMIT BLOCKED: Found references to authUsername or authPassword"
    echo ""
    echo "These variables were removed during session-based auth migration."
    echo "Use session cookies for authentication (they're sent automatically)."
    echo ""
    echo "If you need to add authentication to a fetch call, simply use:"
    echo "  fetch('/api/endpoint')  // Session cookie is sent automatically"
    echo ""
    echo "NOT:"
    echo "  fetch('/api/endpoint', {"
    echo "    headers: { 'Authorization': 'Basic ' + btoa(authUsername + ':' + authPassword) }"
    echo "  })"
    echo ""
    exit 1
fi

exit 0
