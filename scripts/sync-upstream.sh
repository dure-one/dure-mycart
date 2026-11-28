#!/bin/bash
# Selective upstream sync for dure-mycart
# Syncs specific features from nikescar/mycart:main_dure while keeping local changes

set -e

UPSTREAM_REPO="https://github.com/nikescar/mycart.git"
UPSTREAM_BRANCH="main_dure"
REMOTE_NAME="fork"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Upstream Sync Tool ===${NC}"

# Check if fork remote exists
if ! git remote get-url $REMOTE_NAME &>/dev/null; then
    echo "Adding upstream remote..."
    git remote add $REMOTE_NAME $UPSTREAM_REPO
fi

# Fetch upstream
echo "Fetching $UPSTREAM_BRANCH from $REMOTE_NAME..."
git fetch $REMOTE_NAME $UPSTREAM_BRANCH

# Find merge base
MERGE_BASE=$(git merge-base HEAD $REMOTE_NAME/$UPSTREAM_BRANCH)
echo -e "Merge base: ${YELLOW}${MERGE_BASE:0:8}${NC}"

# Show upstream commits not in our branch
echo -e "\n${GREEN}=== Upstream commits not yet applied ===${NC}"
git log --oneline --no-merges $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH

# Count commits
UPSTREAM_COUNT=$(git rev-list --count $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH)
echo -e "\n${YELLOW}${UPSTREAM_COUNT} new commits in upstream${NC}"

# Interactive menu
echo -e "\n${GREEN}Select sync mode:${NC}"
echo "1) Cherry-pick by feature (recommended)"
echo "2) Cherry-pick specific commits"
echo "3) Full merge (may conflict)"
echo "4) Show changed files only"
echo "5) Exit"
read -p "Choice: " choice

case $choice in
    1)
        # Group commits by feature
        echo -e "\n${GREEN}=== Grouping by feature ===${NC}"
        echo -e "\n${YELLOW}SEO commits:${NC}"
        git log --oneline --no-merges $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH | grep -i "seo\|sitemap\|robots" || echo "None"

        echo -e "\n${YELLOW}JWT commits:${NC}"
        git log --oneline --no-merges $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH | grep -i "jwt\|auth\|token" || echo "None"

        echo -e "\n${YELLOW}i18n commits:${NC}"
        git log --oneline --no-merges $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH | grep -i "i18n\|translat\|locale" || echo "None"

        read -p "Enter feature keyword to cherry-pick (seo/jwt/i18n): " keyword

        # Get commits matching keyword
        commits=$(git log --reverse --format=%H --no-merges $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH | \
                  while read hash; do
                      if git log --format=%s -n 1 $hash | grep -qi "$keyword"; then
                          echo $hash
                      fi
                  done)

        if [ -z "$commits" ]; then
            echo "No commits found matching '$keyword'"
            exit 0
        fi

        echo -e "${GREEN}Cherry-picking commits:${NC}"
        for commit in $commits; do
            echo "  - $(git log --format='%h %s' -n 1 $commit)"
        done

        read -p "Proceed? (y/n): " confirm
        if [ "$confirm" = "y" ]; then
            for commit in $commits; do
                echo -e "\n${GREEN}Applying $(git log --format='%h %s' -n 1 $commit)${NC}"
                if ! git cherry-pick $commit; then
                    echo -e "${RED}Conflict detected. Resolve manually, then:${NC}"
                    echo "  git cherry-pick --continue  # after fixing"
                    echo "  git cherry-pick --abort     # to cancel"
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

            echo -e "${GREEN}✓ Feature sync complete${NC}"
        fi
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
        ;;

    3)
        # Full merge
        echo -e "${YELLOW}Warning: This will merge ALL upstream changes${NC}"
        read -p "Continue? (y/n): " confirm
        if [ "$confirm" = "y" ]; then
            git merge --no-ff $REMOTE_NAME/$UPSTREAM_BRANCH -m "merge: sync from $REMOTE_NAME/$UPSTREAM_BRANCH" || {
                echo -e "${RED}Merge conflicts. Resolve manually.${NC}"
                exit 1
            }
        fi
        ;;

    4)
        # Show diff
        echo -e "\n${GREEN}=== Changed files in upstream ===${NC}"
        git diff --name-status $MERGE_BASE..$REMOTE_NAME/$UPSTREAM_BRANCH
        ;;

    *)
        echo "Exiting"
        exit 0
        ;;
esac
