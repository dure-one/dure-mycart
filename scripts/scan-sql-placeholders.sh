#!/usr/bin/env bash
# Script to scan for hardcoded SQLite-style ? placeholders that break PostgreSQL
# Excludes test files and checks for conditional database handling

set -e

echo "🔍 Scanning for hardcoded SQL placeholders..."
echo ""

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Find all Go files (excluding tests and vendor)
FILES=$(find internal -name "*.go" ! -name "*_test.go" -type f)

# Track issues
TOTAL_ISSUES=0
CRITICAL_FILES=0

for file in $FILES; do
    # Skip if file doesn't contain SQL queries
    if ! grep -q "QueryContext\|ExecContext\|PrepareContext\|QueryRowContext" "$file" 2>/dev/null; then
        continue
    fi

    # Look for hardcoded ? placeholders in SQL strings
    LINES=$(grep -n "= \`.*?.*\`\|= \".*?.*\"\|WriteString(\".*?.*\")" "$file" 2>/dev/null | grep -v "//" || true)

    if [ ! -z "$LINES" ]; then
        # Check if file has conditional database handling
        HAS_CONDITIONAL=$(grep -c "DBType()\|if.*postgres\|if.*sqlite" "$file" 2>/dev/null || echo "0")

        FILE_HAS_ISSUES=false

        while IFS= read -r line; do
            LINE_NUM=$(echo "$line" | cut -d: -f1)
            LINE_CONTENT=$(echo "$line" | cut -d: -f2-)

            # Filter out false positives
            # Skip if it's a comment
            if echo "$LINE_CONTENT" | grep -q "^[[:space:]]*//"; then
                continue
            fi

            # Skip if it's in a conditional block (already handled)
            # Check context around the line
            CONTEXT_START=$((LINE_NUM - 5))
            CONTEXT_END=$((LINE_NUM + 2))
            if [ $CONTEXT_START -lt 1 ]; then CONTEXT_START=1; fi

            CONTEXT=$(sed -n "${CONTEXT_START},${CONTEXT_END}p" "$file" 2>/dev/null || echo "")

            # Skip if already in a DBType conditional
            if echo "$CONTEXT" | grep -q "if DBType.*postgres\|if DBType.*sqlite"; then
                continue
            fi

            # Check if the line contains SQL placeholder
            if echo "$LINE_CONTENT" | grep -q "WHERE.*=.*?\|VALUES.*?\|SET.*=.*?\|IN (.*?\|LIMIT ?"; then
                if [ "$FILE_HAS_ISSUES" = false ]; then
                    echo -e "${RED}❌ $file${NC}"
                    if [ "$HAS_CONDITIONAL" -gt 0 ]; then
                        echo -e "   ${YELLOW}⚠️  File has some conditional handling but missed cases:${NC}"
                    else
                        echo -e "   ${RED}⚠️  No conditional database handling found${NC}"
                    fi
                    FILE_HAS_ISSUES=true
                    CRITICAL_FILES=$((CRITICAL_FILES + 1))
                fi

                # Truncate long lines for display
                DISPLAY_LINE=$(echo "$LINE_CONTENT" | cut -c1-80)
                if [ ${#LINE_CONTENT} -gt 80 ]; then
                    DISPLAY_LINE="${DISPLAY_LINE}..."
                fi

                echo -e "   ${YELLOW}Line $LINE_NUM:${NC} $DISPLAY_LINE"
                TOTAL_ISSUES=$((TOTAL_ISSUES + 1))
            fi
        done <<< "$LINES"

        if [ "$FILE_HAS_ISSUES" = true ]; then
            echo ""
        fi
    fi
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ $TOTAL_ISSUES -eq 0 ]; then
    echo -e "${GREEN}✅ No hardcoded SQL placeholders found!${NC}"
else
    echo -e "${RED}Found $TOTAL_ISSUES potential issues in $CRITICAL_FILES files${NC}"
    echo ""
    echo "These queries use SQLite-style ? placeholders which break on PostgreSQL."
    echo "PostgreSQL requires \$1, \$2, \$3... syntax instead."
    echo ""
    echo -e "${YELLOW}Recommended fixes:${NC}"
    echo "1. Add helper function to build database-specific placeholders"
    echo "2. Use conditional query building based on DBType()"
    echo "3. Migrate to sqlc-generated queries where possible"
fi

echo ""
