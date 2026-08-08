#!/usr/bin/env bash
set -euo pipefail

# verify-arch-lint.sh — boundary enforcement proof for P0.13.
#
# A boundary linter nobody has watched fail is a boundary linter nobody trusts.
# This script proves go-arch-lint passes on a clean tree AND rejects a real
# violation — one that compiles and resolves, so the failure can only come from
# the boundary rule and not from an unresolvable import.

LINTER_CMD=${LINTER_CMD:-go-arch-lint}
if ! command -v go-arch-lint >/dev/null 2>&1; then
    LINTER_CMD="go run github.com/fe3dback/go-arch-lint@latest"
fi

# The violating file imports a package that genuinely exists, so `go build`
# succeeds and go-arch-lint has to reject it on the rule alone.
VICTIM_PACKAGE="github.com/fluentra/fluentra/internal/platform/storage"
VIOLATION_DIR="internal/platform/telemetry"
VIOLATION_FILE="${VIOLATION_DIR}/zz_arch_violation_probe.go"

created_dir=""
cleanup() {
    rm -f "$VIOLATION_FILE"
    if [ -n "$created_dir" ]; then rmdir "$created_dir" 2>/dev/null || true; fi
}
trap cleanup EXIT

echo "==> Step 1: architecture lint on the clean tree"
if ! $LINTER_CMD check; then
    echo "FAIL: the clean tree does not pass go-arch-lint."
    exit 1
fi
echo "    clean tree passes"

echo "==> Step 2: the violation must not be a compile error"
if [ ! -d "$VIOLATION_DIR" ]; then
    mkdir -p "$VIOLATION_DIR"
    created_dir="$VIOLATION_DIR"
fi
cat > "$VIOLATION_FILE" <<EOF
package telemetry

// Deliberate L1 violation: platform/telemetry may depend on shared only, so
// importing another platform capability must be rejected. The import resolves
// and compiles — that is the point.
import _ "${VICTIM_PACKAGE}"
EOF

if ! go build ./... >/dev/null 2>&1; then
    echo "FAIL: the probe does not compile, so a linter failure would prove nothing."
    echo "      Pick a package that exists for VICTIM_PACKAGE."
    exit 1
fi
echo "    probe compiles; any failure below is the boundary rule"

echo "==> Step 3: go-arch-lint must reject the violation"
set +e
violation_output=$($LINTER_CMD check 2>&1)
violation_status=$?
set -e

if [ $violation_status -eq 0 ]; then
    echo "FAIL: go-arch-lint accepted a deliberate boundary violation."
    echo "      The rules are not enforcing L1."
    exit 1
fi

# Exit status alone is not proof: a config typo also exits non-zero. Require the
# linter to name the component and the package it refused.
if ! printf '%s' "$violation_output" | grep -qi "p_telemetry"; then
    echo "FAIL: go-arch-lint failed, but not with a boundary violation for p_telemetry."
    echo "      Output was:"
    printf '%s\n' "$violation_output"
    exit 1
fi
if ! printf '%s' "$violation_output" | grep -q "platform/storage"; then
    echo "FAIL: go-arch-lint did not name the forbidden dependency."
    echo "      Output was:"
    printf '%s\n' "$violation_output"
    exit 1
fi
echo "    rejected, naming p_telemetry and the forbidden platform/storage import"

cleanup
trap - EXIT

echo "==> Step 4: the tree is clean again"
if ! $LINTER_CMD check; then
    echo "FAIL: the tree does not pass after cleanup."
    exit 1
fi
if [ -e "$VIOLATION_FILE" ]; then
    echo "FAIL: the probe file was left behind at $VIOLATION_FILE"
    exit 1
fi

echo "SUCCESS: boundary enforcement proven."
