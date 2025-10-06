#!/bin/bash

# Script to verify that schema changes are accompanied by tarball updates
# This helps developers catch issues before pushing to a PR

set -e

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

echo "🔍 Checking for schema and tarball changes..."
echo ""

# Get the default branch (usually main or master)
DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')

# Check if there are uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "⚠️  Warning: You have uncommitted changes. This check will include staged and unstaged changes."
    echo ""
fi

# Check for changes in data/schema/ directory
SCHEMA_CHANGES=$(git diff --name-only "$DEFAULT_BRANCH"...HEAD data/schema/ 2>/dev/null | wc -l)
SCHEMA_CHANGES_STAGED=$(git diff --cached --name-only data/schema/ 2>/dev/null | wc -l)
SCHEMA_CHANGES_UNSTAGED=$(git diff --name-only data/schema/ 2>/dev/null | wc -l)

# Check for changes in resources.tar.gz
TARBALL_CHANGES=$(git diff --name-only "$DEFAULT_BRANCH"...HEAD resources.tar.gz 2>/dev/null | wc -l)
TARBALL_CHANGES_STAGED=$(git diff --cached --name-only resources.tar.gz 2>/dev/null | wc -l)
TARBALL_CHANGES_UNSTAGED=$(git diff --name-only resources.tar.gz 2>/dev/null | wc -l)

TOTAL_SCHEMA_CHANGES=$((SCHEMA_CHANGES + SCHEMA_CHANGES_STAGED + SCHEMA_CHANGES_UNSTAGED))
TOTAL_TARBALL_CHANGES=$((TARBALL_CHANGES + TARBALL_CHANGES_STAGED + TARBALL_CHANGES_UNSTAGED))

echo "📊 Status:"
echo "  Schema files changed: ${TOTAL_SCHEMA_CHANGES}"
echo "  Tarball changed: ${TOTAL_TARBALL_CHANGES}"
echo ""

EXIT_CODE=0

if [ "${TOTAL_SCHEMA_CHANGES}" -gt 0 ] && [ "${TOTAL_TARBALL_CHANGES}" -eq 0 ]; then
    echo "❌ ERROR: Schema files were modified but resources.tar.gz was not updated!"
    echo ""
    echo "📝 To fix this issue:"
    echo "  1. Run: make build-schemas"
    echo "  2. Stage the updated files:"
    echo "     git add resources.tar.gz deploy/kessel-inventory-ephem.yaml"
    echo "  3. Commit the changes"
    echo ""
    EXIT_CODE=1
elif [ "${TOTAL_SCHEMA_CHANGES}" -eq 0 ] && [ "${TOTAL_TARBALL_CHANGES}" -gt 0 ]; then
    echo "⚠️  WARNING: Tarball was modified but no schema files changed."
    echo "   This may be intentional, but please verify this is expected."
    echo ""
elif [ "${TOTAL_SCHEMA_CHANGES}" -gt 0 ] && [ "${TOTAL_TARBALL_CHANGES}" -gt 0 ]; then
    echo "✅ SUCCESS: Both schema files and tarball have been updated."
    echo ""
    
    # Additional check: verify tarball is up-to-date by comparing timestamps
    NEWEST_SCHEMA=$(find data/schema/resources -type f -printf '%T@\n' 2>/dev/null | sort -n | tail -1)
    TARBALL_TIME=$(stat -c '%Y' resources.tar.gz 2>/dev/null || echo "0")
    
    if [ -n "$NEWEST_SCHEMA" ] && [ "$(echo "$NEWEST_SCHEMA > $TARBALL_TIME" | bc 2>/dev/null || echo "0")" -eq 1 ]; then
        echo "⚠️  WARNING: Some schema files are newer than the tarball."
        echo "   Consider running 'make build-schemas' to ensure the tarball is fully up-to-date."
        echo ""
    fi
else
    echo "✅ No schema or tarball changes detected."
    echo ""
fi

exit $EXIT_CODE
