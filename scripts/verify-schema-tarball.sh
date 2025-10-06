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

# Check for changes in schema_cache.json
CACHE_CHANGES=$(git diff --name-only "$DEFAULT_BRANCH"...HEAD schema_cache.json 2>/dev/null | wc -l)
CACHE_CHANGES_STAGED=$(git diff --cached --name-only schema_cache.json 2>/dev/null | wc -l)
CACHE_CHANGES_UNSTAGED=$(git diff --name-only schema_cache.json 2>/dev/null | wc -l)

TOTAL_SCHEMA_CHANGES=$((SCHEMA_CHANGES + SCHEMA_CHANGES_STAGED + SCHEMA_CHANGES_UNSTAGED))
TOTAL_TARBALL_CHANGES=$((TARBALL_CHANGES + TARBALL_CHANGES_STAGED + TARBALL_CHANGES_UNSTAGED))
TOTAL_CACHE_CHANGES=$((CACHE_CHANGES + CACHE_CHANGES_STAGED + CACHE_CHANGES_UNSTAGED))

echo "📊 Status:"
echo "  Schema files changed: ${TOTAL_SCHEMA_CHANGES}"
echo "  Tarball changed: ${TOTAL_TARBALL_CHANGES}"
echo "  Schema cache changed: ${TOTAL_CACHE_CHANGES}"
echo ""

EXIT_CODE=0
ERRORS=()

if [ "${TOTAL_SCHEMA_CHANGES}" -gt 0 ]; then
    # Schema files changed - check if generated files are also updated
    if [ "${TOTAL_TARBALL_CHANGES}" -eq 0 ]; then
        ERRORS+=("resources.tar.gz was not updated")
    fi
    
    if [ "${TOTAL_CACHE_CHANGES}" -eq 0 ]; then
        ERRORS+=("schema_cache.json was not updated")
    fi
    
    if [ ${#ERRORS[@]} -gt 0 ]; then
        echo "❌ ERROR: Schema files were modified but generated files were not updated!"
        echo ""
        for error in "${ERRORS[@]}"; do
            echo "   - ${error}"
        done
        echo ""
        echo "📝 To fix this issue:"
        echo "  1. Run: make build-schemas"
        echo "  2. Run: go run main.go preload-schema"
        echo "  3. Stage the updated files:"
        echo "     git add resources.tar.gz schema_cache.json deploy/kessel-inventory-ephem.yaml"
        echo "  4. Commit the changes"
        echo ""
        EXIT_CODE=1
    else
        echo "✅ SUCCESS: Schema files and generated files have been updated."
        echo ""
        
        # Additional check: verify files are up-to-date by comparing timestamps
        NEWEST_SCHEMA=$(find data/schema/resources -type f -printf '%T@\n' 2>/dev/null | sort -n | tail -1)
        TARBALL_TIME=$(stat -c '%Y' resources.tar.gz 2>/dev/null || echo "0")
        CACHE_TIME=$(stat -c '%Y' schema_cache.json 2>/dev/null || echo "0")
        
        STALE_FILES=()
        if [ -n "$NEWEST_SCHEMA" ]; then
            if [ "$(echo "$NEWEST_SCHEMA > $TARBALL_TIME" | bc 2>/dev/null || echo "0")" -eq 1 ]; then
                STALE_FILES+=("resources.tar.gz")
            fi
            if [ "$(echo "$NEWEST_SCHEMA > $CACHE_TIME" | bc 2>/dev/null || echo "0")" -eq 1 ]; then
                STALE_FILES+=("schema_cache.json")
            fi
        fi
        
        if [ ${#STALE_FILES[@]} -gt 0 ]; then
            echo "⚠️  WARNING: Some schema files are newer than the generated files:"
            for file in "${STALE_FILES[@]}"; do
                echo "   - ${file} appears stale"
            done
            echo ""
            echo "   Consider rebuilding to ensure all files are up-to-date:"
            echo "   - make build-schemas"
            echo "   - go run main.go preload-schema"
            echo ""
        fi
    fi
elif [ "${TOTAL_TARBALL_CHANGES}" -gt 0 ] || [ "${TOTAL_CACHE_CHANGES}" -gt 0 ]; then
    echo "⚠️  WARNING: Generated files were modified but no schema files changed."
    echo "   This may be intentional, but please verify this is expected."
    echo ""
else
    echo "✅ No schema or generated file changes detected."
    echo ""
fi

exit $EXIT_CODE
