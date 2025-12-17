#!/bin/bash
# Coverage script for bv
# Usage:
#   ./scripts/coverage.sh          # Run coverage and show summary
#   ./scripts/coverage.sh html     # Generate and open HTML report
#   ./scripts/coverage.sh check    # Check against thresholds (CI mode)
#   ./scripts/coverage.sh pkg      # Show per-package breakdown
#   ./scripts/coverage.sh uncovered # Show uncovered lines

set -e

COVERAGE_DIR="coverage"
COVERAGE_FILE="$COVERAGE_DIR/coverage.out"
HTML_FILE="$COVERAGE_DIR/coverage.html"

# Per-package thresholds (match CI)
declare -A THRESHOLDS=(
    ["github.com/Dicklesworthstone/beads_viewer/pkg/analysis"]=75
    ["github.com/Dicklesworthstone/beads_viewer/pkg/export"]=95
    ["github.com/Dicklesworthstone/beads_viewer/pkg/recipe"]=90
    ["github.com/Dicklesworthstone/beads_viewer/pkg/ui"]=55
    ["github.com/Dicklesworthstone/beads_viewer/pkg/loader"]=80
    ["github.com/Dicklesworthstone/beads_viewer/pkg/updater"]=70
    ["github.com/Dicklesworthstone/beads_viewer/pkg/watcher"]=80
    ["github.com/Dicklesworthstone/beads_viewer/pkg/workspace"]=85
)

PROJECT_THRESHOLD=60

mkdir -p "$COVERAGE_DIR"

run_coverage() {
    echo "Running tests with coverage..."
    go test -covermode=atomic -coverprofile="$COVERAGE_FILE" ./... 2>&1 | grep -v "^?"
    echo ""
}

show_summary() {
    echo "=== Coverage Summary ==="
    total=$(go tool cover -func="$COVERAGE_FILE" | tail -1)
    echo "$total"
    echo ""
}

show_per_package() {
    echo "=== Per-Package Coverage ==="
    go tool cover -func="$COVERAGE_FILE" | grep -E '^github.com/Dicklesworthstone/beads_viewer/pkg/[^/]+\)' | \
        awk '{gsub(/github.com\/Dicklesworthstone\/beads_viewer\//, ""); print $1, $3}' | \
        column -t
    echo ""
}

show_detailed() {
    echo "=== Detailed Function Coverage ==="
    go tool cover -func="$COVERAGE_FILE" | head -50
    echo "..."
    echo "(Use './scripts/coverage.sh html' for full report)"
    echo ""
}

show_uncovered() {
    echo "=== Uncovered Lines ==="
    echo "Generating uncovered lines report..."

    # Parse coverage.out for lines with 0 coverage
    awk -F':' '
    /\.go:/ && !/mode:/ {
        split($2, parts, ",")
        line_info = parts[1]
        split(line_info, line_parts, " ")
        count = line_parts[2]
        if (count == "0") {
            file = $1
            gsub(/github.com\/Dicklesworthstone\/beads_viewer\//, "", file)
            print file ":" parts[1]
        }
    }' "$COVERAGE_FILE" | head -30

    echo ""
    echo "(Showing first 30 uncovered sections)"
}

generate_html() {
    echo "Generating HTML coverage report..."
    go tool cover -html="$COVERAGE_FILE" -o "$HTML_FILE"
    echo "Report generated: $HTML_FILE"

    # Open in browser if possible
    if command -v open &> /dev/null; then
        open "$HTML_FILE"
    elif command -v xdg-open &> /dev/null; then
        xdg-open "$HTML_FILE"
    else
        echo "Open $HTML_FILE in your browser"
    fi
}

check_thresholds() {
    echo "=== Checking Coverage Thresholds ==="
    local failed=0

    # Check project threshold
    total=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}' | tr -d '%')
    if (( $(echo "$total < $PROJECT_THRESHOLD" | bc -l) )); then
        echo "FAIL: Total coverage ${total}% < ${PROJECT_THRESHOLD}%"
        failed=1
    else
        echo "PASS: Total coverage ${total}% >= ${PROJECT_THRESHOLD}%"
    fi

    # Check per-package thresholds
    go tool cover -func="$COVERAGE_FILE" | grep '^github.com/Dicklesworthstone/beads_viewer/pkg/' | \
    while read -r line; do
        pkg=$(echo "$line" | awk '{print $1}')
        # Extract base package (without function name)
        base_pkg=$(echo "$pkg" | sed 's/)[^)]*$/)/; s/\/[^/]*$//')
        pct=$(echo "$line" | awk '{print $3}' | tr -d '%')

        threshold=${THRESHOLDS[$base_pkg]}
        if [ -n "$threshold" ]; then
            if (( $(echo "$pct < $threshold" | bc -l) )); then
                echo "FAIL: $base_pkg ${pct}% < ${threshold}%"
                failed=1
            fi
        fi
    done

    if [ $failed -eq 0 ]; then
        echo ""
        echo "All coverage thresholds passed!"
        return 0
    else
        echo ""
        echo "Some coverage thresholds failed!"
        return 1
    fi
}

# Main
case "${1:-summary}" in
    html)
        run_coverage
        generate_html
        ;;
    check)
        run_coverage
        check_thresholds
        ;;
    pkg|package)
        run_coverage
        show_per_package
        ;;
    uncovered)
        run_coverage
        show_uncovered
        ;;
    detailed)
        run_coverage
        show_detailed
        ;;
    summary|*)
        run_coverage
        show_summary
        show_per_package
        ;;
esac
