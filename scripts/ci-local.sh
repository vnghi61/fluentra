#!/usr/bin/env bash
# Runs the ci-backend gates inside the Linux container, reporting each one's
# exit code rather than stopping at the first failure.
#
# It exists because the equivalent one-liner has to be quoted through
# PowerShell, and the quoting mangles it often enough to waste a run. Invoke it
# from the container command in docs/development/HANDOFF-WP2.md §3.
set -u

run() {
	local name="$1"
	shift
	if "$@" >"/tmp/${name}.log" 2>&1; then
		echo "PASS  ${name}"
	else
		echo "FAIL  ${name}"
		grep -E "^(FAIL|Error|error)" "/tmp/${name}.log" | head -15
	fi
}

run arch make arch
run vet make vet
run test make test
run test-int make test-int
run test-contract make test-contract
