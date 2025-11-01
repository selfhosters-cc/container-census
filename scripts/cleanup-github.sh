#!/bin/bash

# GitHub Cleanup Script
# Interactively delete old releases and packages

# Note: Not using set -e because interactive read commands can return non-zero
# which would cause script to exit prematurely

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="selfhosters-cc/container-census"
ORG="selfhosters-cc"

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
    echo "Install with: sudo apt install gh"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: Not authenticated with GitHub${NC}"
    echo "Run: gh auth login"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is not installed${NC}"
    echo "Install with: sudo apt install jq"
    exit 1
fi

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  GitHub Cleanup Script                 ║${NC}"
echo -e "${BLUE}║  Repository: ${REPO}  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo

# Function to cleanup releases
cleanup_releases() {
    echo -e "${YELLOW}═══ GitHub Releases ═══${NC}"
    echo

    # Get all releases
    releases=$(gh release list --repo "$REPO" --limit 1000 --json tagName,name,createdAt,isLatest | jq -r '.[] | "\(.tagName)|\(.name)|\(.createdAt)|\(.isLatest)"')

    if [ -z "$releases" ]; then
        echo -e "${YELLOW}No releases found${NC}"
        return
    fi

    release_count=$(echo "$releases" | wc -l)
    echo -e "${GREEN}Found $release_count releases${NC}"
    echo

    # Show options
    echo "What would you like to do?"
    echo "  1) Keep only the latest release (delete all others)"
    echo "  2) Keep the latest N releases (interactive)"
    echo "  3) Review each release interactively"
    echo "  4) Skip release cleanup"
    echo
    read -p "Enter choice [1-4]: " choice

    case $choice in
        1)
            echo
            echo -e "${YELLOW}Keeping only the latest release...${NC}"
            deleted=0
            while IFS='|' read -r tag name created_at is_latest; do
                if [ "$is_latest" != "true" ]; then
                    echo -e "${RED}Deleting: $tag - $name (created: $created_at)${NC}"
                    gh release delete "$tag" --repo "$REPO" --yes
                    ((deleted++))
                else
                    echo -e "${GREEN}Keeping: $tag - $name (LATEST)${NC}"
                fi
            done <<< "$releases"
            echo
            echo -e "${GREEN}Deleted $deleted releases${NC}"
            ;;

        2)
            echo
            read -p "How many recent releases to keep? " keep_count

            if ! [[ "$keep_count" =~ ^[0-9]+$ ]]; then
                echo -e "${RED}Invalid number${NC}"
                return
            fi

            echo
            echo -e "${YELLOW}Keeping the latest $keep_count releases...${NC}"
            deleted=0
            index=0

            while IFS='|' read -r tag name created_at is_latest; do
                ((index++))
                if [ $index -le $keep_count ]; then
                    echo -e "${GREEN}Keeping: $tag - $name (created: $created_at)${NC}"
                else
                    echo -e "${RED}Deleting: $tag - $name (created: $created_at)${NC}"
                    gh release delete "$tag" --repo "$REPO" --yes
                    ((deleted++))
                fi
            done <<< "$releases"
            echo
            echo -e "${GREEN}Deleted $deleted releases${NC}"
            ;;

        3)
            echo
            deleted=0
            kept=0
            while IFS='|' read -r tag name created_at is_latest; do
                echo -e "${BLUE}────────────────────────────────${NC}"
                echo -e "Tag:     ${YELLOW}$tag${NC}"
                echo -e "Name:    $name"
                echo -e "Created: $created_at"
                if [ "$is_latest" = "true" ]; then
                    echo -e "Status:  ${GREEN}LATEST${NC}"
                fi
                echo
                read -p "Delete this release? [y/N]: " confirm </dev/tty
                if [[ $confirm =~ ^[Yy]$ ]]; then
                    echo -e "${RED}Deleting...${NC}"
                    gh release delete "$tag" --repo "$REPO" --yes
                    ((deleted++))
                else
                    echo -e "${GREEN}Keeping${NC}"
                    ((kept++))
                fi
                echo
            done <<< "$releases"
            echo -e "${GREEN}Deleted: $deleted | Kept: $kept${NC}"
            ;;

        4)
            echo -e "${YELLOW}Skipping release cleanup${NC}"
            ;;

        *)
            echo -e "${RED}Invalid choice${NC}"
            ;;
    esac
}

# Function to cleanup packages
cleanup_packages() {
    echo
    echo -e "${YELLOW}═══ GitHub Packages (Docker Images) ═══${NC}"
    echo

    # Get package names
    packages=$(gh api \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28" \
        "/orgs/$ORG/packages?package_type=container" \
        | jq -r '.[].name' | sort -u)

    if [ -z "$packages" ]; then
        echo -e "${YELLOW}No packages found${NC}"
        return
    fi

    package_count=$(echo "$packages" | wc -l)
    echo -e "${GREEN}Found $package_count packages:${NC}"
    echo "$packages" | sed 's/^/  - /'
    echo

    # Process each package
    for package in $packages; do
        echo
        echo -e "${BLUE}═══ Package: $package ═══${NC}"

        # Get all versions for this package
        versions=$(gh api \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            "/orgs/$ORG/packages/container/$package/versions" \
            | jq -r '.[] | "\(.id)|\(.name // "untagged")|\(.created_at)|\(.metadata.container.tags // [] | join(","))"')

        if [ -z "$versions" ]; then
            echo -e "${YELLOW}No versions found for $package${NC}"
            continue
        fi

        version_count=$(echo "$versions" | wc -l)
        echo -e "${GREEN}Found $version_count versions${NC}"
        echo

        # Show options for this package
        echo "What would you like to do with $package?"
        echo "  1) Keep only the latest version (delete all others)"
        echo "  2) Keep the latest N versions (interactive)"
        echo "  3) Review each version interactively"
        echo "  4) Delete ALL versions of this package"
        echo "  5) Skip this package"
        echo
        read -p "Enter choice [1-5]: " choice

        case $choice in
            1)
                echo
                echo -e "${YELLOW}Keeping only the latest version...${NC}"
                deleted=0
                index=0

                while IFS='|' read -r id name created_at tags; do
                    ((index++))
                    if [ $index -eq 1 ]; then
                        echo -e "${GREEN}Keeping: $name (tags: $tags, created: $created_at)${NC}"
                    else
                        echo -e "${RED}Deleting: $name (tags: $tags, created: $created_at)${NC}"
                        gh api \
                            --method DELETE \
                            -H "Accept: application/vnd.github+json" \
                            -H "X-GitHub-Api-Version: 2022-11-28" \
                            "/orgs/$ORG/packages/container/$package/versions/$id"
                        ((deleted++))
                    fi
                done <<< "$versions"
                echo
                echo -e "${GREEN}Deleted $deleted versions${NC}"
                ;;

            2)
                echo
                read -p "How many recent versions to keep? " keep_count

                if ! [[ "$keep_count" =~ ^[0-9]+$ ]]; then
                    echo -e "${RED}Invalid number${NC}"
                    continue
                fi

                echo
                echo -e "${YELLOW}Keeping the latest $keep_count versions...${NC}"
                deleted=0
                index=0

                while IFS='|' read -r id name created_at tags; do
                    ((index++))
                    if [ $index -le $keep_count ]; then
                        echo -e "${GREEN}Keeping: $name (tags: $tags, created: $created_at)${NC}"
                    else
                        echo -e "${RED}Deleting: $name (tags: $tags, created: $created_at)${NC}"
                        gh api \
                            --method DELETE \
                            -H "Accept: application/vnd.github+json" \
                            -H "X-GitHub-Api-Version: 2022-11-28" \
                            "/orgs/$ORG/packages/container/$package/versions/$id"
                        ((deleted++))
                    fi
                done <<< "$versions"
                echo
                echo -e "${GREEN}Deleted $deleted versions${NC}"
                ;;

            3)
                echo
                deleted=0
                kept=0
                while IFS='|' read -r id name created_at tags; do
                    echo -e "${BLUE}────────────────────────────────${NC}"
                    echo -e "Version: ${YELLOW}$name${NC}"
                    echo -e "Tags:    $tags"
                    echo -e "Created: $created_at"
                    echo -e "ID:      $id"
                    echo
                    read -p "Delete this version? [y/N]: " confirm </dev/tty
                    if [[ $confirm =~ ^[Yy]$ ]]; then
                        echo -e "${RED}Deleting...${NC}"
                        gh api \
                            --method DELETE \
                            -H "Accept: application/vnd.github+json" \
                            -H "X-GitHub-Api-Version: 2022-11-28" \
                            "/orgs/$ORG/packages/container/$package/versions/$id"
                        ((deleted++))
                    else
                        echo -e "${GREEN}Keeping${NC}"
                        ((kept++))
                    fi
                    echo
                done <<< "$versions"
                echo -e "${GREEN}Deleted: $deleted | Kept: $kept${NC}"
                ;;

            4)
                echo
                echo -e "${RED}⚠️  WARNING: This will delete ALL versions of $package${NC}"
                read -p "Are you absolutely sure? Type 'DELETE' to confirm: " confirm
                if [ "$confirm" = "DELETE" ]; then
                    echo -e "${RED}Deleting all versions...${NC}"
                    deleted=0
                    while IFS='|' read -r id name created_at tags; do
                        echo -e "${RED}Deleting: $name (tags: $tags)${NC}"
                        gh api \
                            --method DELETE \
                            -H "Accept: application/vnd.github+json" \
                            -H "X-GitHub-Api-Version: 2022-11-28" \
                            "/orgs/$ORG/packages/container/$package/versions/$id"
                        ((deleted++))
                    done <<< "$versions"
                    echo
                    echo -e "${GREEN}Deleted $deleted versions (entire package)${NC}"
                else
                    echo -e "${YELLOW}Cancelled${NC}"
                fi
                ;;

            5)
                echo -e "${YELLOW}Skipping $package${NC}"
                ;;

            *)
                echo -e "${RED}Invalid choice${NC}"
                ;;
        esac
    done
}

# Main menu
echo "What would you like to cleanup?"
echo "  1) GitHub Releases only"
echo "  2) GitHub Packages (Docker images) only"
echo "  3) Both releases and packages"
echo "  4) Exit"
echo
read -p "Enter choice [1-4]: " main_choice

case $main_choice in
    1)
        cleanup_releases
        ;;
    2)
        cleanup_packages
        ;;
    3)
        cleanup_releases
        cleanup_packages
        ;;
    4)
        echo -e "${YELLOW}Exiting${NC}"
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

echo
echo -e "${GREEN}✓ Cleanup complete!${NC}"
