#!/bin/bash
# Selective upstream sync for dure-mycart
# Syncs commits from nikescar/mycart:main_dure since last sync

set -e

UPSTREAM_REPO="https://github.com/nikescar/mycart.git"
UPSTREAM_BRANCH="main_dure"
REMOTE_NAME="fork"
SYNC_MARKER=".git/UPSTREAM_SYNC_MARKER"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Upstream Sync Tool ===${NC}"

# Check if fork remote exists
if ! git remote get-url $REMOTE_NAME &>/dev/null; then
    echo "Adding upstream remote..."
    git remote add $REMOTE_NAME $UPSTREAM_REPO
fi

# Fetch upstream
echo "Fetching $UPSTREAM_BRANCH from $REMOTE_NAME..."
git fetch $REMOTE_NAME $UPSTREAM_BRANCH

# Get last synced commit
if [ -f "$SYNC_MARKER" ]; then
    LAST_SYNC=$(cat "$SYNC_MARKER")
    echo -e "Last sync: ${YELLOW}${LAST_SYNC:0:8}${NC}"
else
    # First sync - use merge base
    LAST_SYNC=$(git merge-base HEAD $REMOTE_NAME/$UPSTREAM_BRANCH)
    echo -e "First sync. Using merge base: ${YELLOW}${LAST_SYNC:0:8}${NC}"
fi

# Get new commits
NEW_COMMITS=$(git log --reverse --format=%H --no-merges $LAST_SYNC..$REMOTE_NAME/$UPSTREAM_BRANCH)

if [ -z "$NEW_COMMITS" ]; then
    echo -e "${GREEN}✓ Already up to date${NC}"
    exit 0
fi

# Count and show commits
COMMIT_COUNT=$(echo "$NEW_COMMITS" | wc -l)
echo -e "\n${GREEN}=== ${COMMIT_COUNT} new commits ===${NC}"
git log --oneline --no-merges $LAST_SYNC..$REMOTE_NAME/$UPSTREAM_BRANCH

# Interactive menu
echo -e "\n${GREEN}Select action:${NC}"
echo "1) Apply all new commits"
echo "2) Cherry-pick specific commits"
echo "3) Show changed files only"
echo "4) Exit"
read -p "Choice: " choice

case $choice in
    1)
        # Apply all commits
        echo -e "${GREEN}Cherry-picking ${COMMIT_COUNT} commits...${NC}"

        for commit in $NEW_COMMITS; do
            echo -e "\n${GREEN}Applying $(git log --format='%h %s' -n 1 $commit)${NC}"
            if ! git cherry-pick $commit; then
                echo -e "${RED}Conflict detected. Options:${NC}"
                echo "  git cherry-pick --continue  # after fixing"
                echo "  git cherry-pick --skip      # skip this commit"
                echo "  git cherry-pick --abort     # cancel entire sync"
                exit 1
            fi
        done

        # Fix import paths
        echo -e "\n${GREEN}Fixing import paths...${NC}"
        find . -name "*.go" -type f -exec sed -i 's|"github.com/shurco/mycart/|"github.com/dure-one/dure-mycart/|g' {} +

        if git diff --quiet; then
            echo "No import path changes needed"
        else
            git add -A
            git commit -m "fix: update imports to fork module path after upstream sync"
        fi

        # Update sync marker
        UPSTREAM_HEAD=$(git rev-parse $REMOTE_NAME/$UPSTREAM_BRANCH)
        echo "$UPSTREAM_HEAD" > "$SYNC_MARKER"
        echo -e "\n${GREEN}✓ Sync complete. Marker updated to ${UPSTREAM_HEAD:0:8}${NC}"
        ;;

    2)
        # Manual commit selection
        read -p "Enter commit hash(es) separated by space: " hashes
        for hash in $hashes; do
            echo -e "${GREEN}Cherry-picking $hash${NC}"
            git cherry-pick $hash || {
                echo -e "${RED}Fix conflicts, then run: git cherry-pick --continue${NC}"
                exit 1
            }
        done

        # Don't update marker for manual selection
        echo -e "${YELLOW}Note: Sync marker not updated (manual selection)${NC}"
        ;;

    3)
        # Show diff
        echo -e "\n${GREEN}=== Changed files ===${NC}"
        git diff --name-status $LAST_SYNC..$REMOTE_NAME/$UPSTREAM_BRANCH
        ;;

    *)
        echo "Exiting"
        exit 0
        ;;
esac
