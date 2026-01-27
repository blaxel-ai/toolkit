#!/bin/bash

echo "========================================="
echo "Testing GET command for all resources"
echo "========================================="

go run main.go login $BL_WORKSPACE

# Counters for test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Function to test a resource type
test_resource() {
    local plural=$1
    local singular=$2

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo ""
    echo "Testing $plural..."
    echo "-----------------------------------------"

    # List resources in JSON format
    echo "1. Listing $plural..."
    LIST_OUTPUT=$(go run main.go get $plural -o json 2>&1)
    LIST_EXIT_CODE=$?

    if [ $LIST_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list $plural"
        if echo "$LIST_OUTPUT" | grep -q "Forbidden\|authentication\|Unauthorized"; then
            echo "   Reason: Authentication/Permission error"
            SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        else
            echo "   Error: $(echo "$LIST_OUTPUT" | tail -5)"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        return 1
    fi

    echo "   ✓ Successfully listed $plural"

    # Special handling for images: need resourceType/imageName format
    if [ "$singular" = "image" ]; then
        # Extract resourceType and name for images
        RESOURCE_TYPE=$(echo "$LIST_OUTPUT" | jq -r '.[0].metadata.resourceType // empty' 2>/dev/null)
        FIRST_NAME=$(echo "$LIST_OUTPUT" | jq -r '.[0].metadata.name // empty' 2>/dev/null)

        if [ -z "$FIRST_NAME" ] || [ -z "$RESOURCE_TYPE" ]; then
            echo "   ⚠️  No $plural found or unable to extract name"
            echo "   Status: List succeeded but no resources exist"
            SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            return 0
        fi

        # Construct the image reference
        IMAGE_REF="${RESOURCE_TYPE}/${FIRST_NAME}"
        echo "   Found first $singular: $IMAGE_REF"

        # Get the specific resource by resourceType/name
        echo "2. Getting specific $singular: $IMAGE_REF..."
        GET_OUTPUT=$(go run main.go get $singular $IMAGE_REF -o json 2>&1)
        GET_EXIT_CODE=$?

        if [ $GET_EXIT_CODE -ne 0 ]; then
            echo "   ❌ Failed to get $singular $IMAGE_REF"
            if echo "$GET_OUTPUT" | grep -q "Forbidden\|authentication\|Unauthorized"; then
                echo "   Reason: Authentication/Permission error"
                SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            else
                echo "   Error: $(echo "$GET_OUTPUT" | tail -5)"
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            return 1
        fi

        echo "   ✓ Successfully retrieved $singular: $IMAGE_REF"

        # Verify the resourceType and name match (response is wrapped in an array)
        RETRIEVED_TYPE=$(echo "$GET_OUTPUT" | jq -r '.[0].metadata.resourceType // empty' 2>/dev/null)
        RETRIEVED_NAME=$(echo "$GET_OUTPUT" | jq -r '.[0].metadata.name // empty' 2>/dev/null)

        if [ "$RETRIEVED_TYPE" = "$RESOURCE_TYPE" ] && [ "$RETRIEVED_NAME" = "$FIRST_NAME" ]; then
            echo "   ✓ Name verification passed"
            echo "   ✅ All checks passed for $plural"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "   ⚠️  Mismatch: expected $IMAGE_REF, got $RETRIEVED_TYPE/$RETRIEVED_NAME"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
    else
        # Standard handling for other resources
        FIRST_NAME=$(echo "$LIST_OUTPUT" | jq -r '.[0].metadata.name // empty' 2>/dev/null)

        if [ -z "$FIRST_NAME" ]; then
            echo "   ⚠️  No $plural found or unable to extract name"
            echo "   Status: List succeeded but no resources exist"
            SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            return 0
        fi

        echo "   Found first $singular: $FIRST_NAME"

        # Get the specific resource by name
        echo "2. Getting specific $singular: $FIRST_NAME..."
        GET_OUTPUT=$(go run main.go get $singular $FIRST_NAME -o json 2>&1)
        GET_EXIT_CODE=$?

        if [ $GET_EXIT_CODE -ne 0 ]; then
            echo "   ❌ Failed to get $singular $FIRST_NAME"
            if echo "$GET_OUTPUT" | grep -q "Forbidden\|authentication\|Unauthorized"; then
                echo "   Reason: Authentication/Permission error"
                SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            else
                echo "   Error: $(echo "$GET_OUTPUT" | tail -5)"
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            return 1
        fi

        echo "   ✓ Successfully retrieved $singular: $FIRST_NAME"

        # Verify the name matches (response is wrapped in an array)
        RETRIEVED_NAME=$(echo "$GET_OUTPUT" | jq -r '.[0].metadata.name // empty' 2>/dev/null)

        if [ "$RETRIEVED_NAME" = "$FIRST_NAME" ]; then
            echo "   ✓ Name verification passed"
            echo "   ✅ All checks passed for $plural"
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            echo "   ⚠️  Name mismatch: expected $FIRST_NAME, got $RETRIEVED_NAME"
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
    fi

    return 0
}

# Function to test sandbox processes (nested resource)
test_sandbox_processes() {
    local sandbox_name=$1

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo ""
    echo "Testing sandbox processes for $sandbox_name..."
    echo "-----------------------------------------"

    # First, start a process on the sandbox
    echo "1. Starting a process on sandbox $sandbox_name..."
    RUN_OUTPUT=$(go run main.go run sandbox $sandbox_name --path /process -d '{"command": "echo hello"}' 2>&1)
    RUN_EXIT_CODE=$?

    if [ $RUN_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to start process on sandbox $sandbox_name"
        echo "   Error: $(echo "$RUN_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully started process"

    # List processes using plural form
    echo "2. Listing processes (plural form)..."
    LIST_OUTPUT=$(go run main.go get sandbox $sandbox_name processes -o json 2>&1)
    LIST_EXIT_CODE=$?

    if [ $LIST_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list processes (plural)"
        echo "   Error: $(echo "$LIST_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully listed processes (plural form)"

    # List processes using singular form
    echo "3. Listing processes (singular form)..."
    LIST_SINGULAR_OUTPUT=$(go run main.go get sandbox $sandbox_name process -o json 2>&1)
    LIST_SINGULAR_EXIT_CODE=$?

    if [ $LIST_SINGULAR_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list processes (singular)"
        echo "   Error: $(echo "$LIST_SINGULAR_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully listed processes (singular form)"

    # List processes using short form (ps)
    echo "4. Listing processes (short form 'ps')..."
    LIST_PS_OUTPUT=$(go run main.go get sandbox $sandbox_name ps -o json 2>&1)
    LIST_PS_EXIT_CODE=$?

    if [ $LIST_PS_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list processes (ps)"
        echo "   Error: $(echo "$LIST_PS_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully listed processes (short form 'ps')"

    # Extract first process name
    # Note: jq may fail due to unescaped control characters in logs field, so use grep as fallback
    FIRST_PROCESS=$(echo "$LIST_OUTPUT" | jq -r '.[0].name // empty' 2>/dev/null)
    if [ -z "$FIRST_PROCESS" ]; then
        # Fallback: use grep to extract the first "name" field
        FIRST_PROCESS=$(echo "$LIST_OUTPUT" | grep -o '"name": *"[^"]*"' | head -1 | sed 's/.*: *"\([^"]*\)"/\1/')
    fi

    if [ -z "$FIRST_PROCESS" ]; then
        echo "   ⚠️  No processes found"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        return 0
    fi

    echo "   Found process: $FIRST_PROCESS"

    # Get specific process
    echo "5. Getting specific process: $FIRST_PROCESS..."
    GET_OUTPUT=$(go run main.go get sandbox $sandbox_name process $FIRST_PROCESS -o json 2>&1)
    GET_EXIT_CODE=$?

    if [ $GET_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to get process $FIRST_PROCESS"
        echo "   Error: $(echo "$GET_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully retrieved process: $FIRST_PROCESS"
    echo "   ✅ All checks passed for sandbox processes"
    PASSED_TESTS=$((PASSED_TESTS + 1))

    return 0
}

# Function to test job executions (nested resource)
test_job_executions() {
    local job_name=$1

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo ""
    echo "Testing job executions for $job_name..."
    echo "-----------------------------------------"

    # First, start a job execution
    echo "1. Starting a job execution on $job_name..."
    RUN_OUTPUT=$(go run main.go run job $job_name -d '{"tasks": [{"name": "test-task"}]}' 2>&1)
    RUN_EXIT_CODE=$?

    if [ $RUN_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to start job execution on $job_name"
        echo "   Error: $(echo "$RUN_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully started job execution"

    # Give the execution a moment to register
    sleep 2

    # List executions using plural form
    echo "2. Listing executions (plural form)..."
    LIST_OUTPUT=$(go run main.go get job $job_name executions -o json 2>&1)
    LIST_EXIT_CODE=$?

    if [ $LIST_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list executions (plural)"
        echo "   Error: $(echo "$LIST_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully listed executions (plural form)"

    # List executions using singular form
    echo "3. Listing executions (singular form)..."
    LIST_SINGULAR_OUTPUT=$(go run main.go get job $job_name execution -o json 2>&1)
    LIST_SINGULAR_EXIT_CODE=$?

    if [ $LIST_SINGULAR_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to list executions (singular)"
        echo "   Error: $(echo "$LIST_SINGULAR_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully listed executions (singular form)"

    # Extract first execution ID
    FIRST_EXECUTION=$(echo "$LIST_OUTPUT" | jq -r '.[0].metadata.id // empty' 2>/dev/null)

    if [ -z "$FIRST_EXECUTION" ]; then
        echo "   ⚠️  No executions found"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        return 0
    fi

    echo "   Found execution: $FIRST_EXECUTION"

    # Get specific execution (plural form)
    echo "4. Getting specific execution (plural form): $FIRST_EXECUTION..."
    GET_OUTPUT=$(go run main.go get job $job_name executions $FIRST_EXECUTION -o json 2>&1)
    GET_EXIT_CODE=$?

    if [ $GET_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to get execution $FIRST_EXECUTION (plural)"
        echo "   Error: $(echo "$GET_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully retrieved execution (plural form)"

    # Get specific execution (singular form)
    echo "5. Getting specific execution (singular form): $FIRST_EXECUTION..."
    GET_SINGULAR_OUTPUT=$(go run main.go get job $job_name execution $FIRST_EXECUTION -o json 2>&1)
    GET_SINGULAR_EXIT_CODE=$?

    if [ $GET_SINGULAR_EXIT_CODE -ne 0 ]; then
        echo "   ❌ Failed to get execution $FIRST_EXECUTION (singular)"
        echo "   Error: $(echo "$GET_SINGULAR_OUTPUT" | tail -5)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo "   ✓ Successfully retrieved execution (singular form)"
    echo "   ✅ All checks passed for job executions"
    PASSED_TESTS=$((PASSED_TESTS + 1))

    return 0
}

# Test all resource types
test_resource "policies" "policy"
test_resource "models" "model"
test_resource "functions" "function"
test_resource "agents" "agent"
test_resource "integrationconnections" "integrationconnection"
test_resource "sandboxes" "sandbox"
test_resource "jobs" "job"
test_resource "volumes" "volume"
test_resource "volumetemplates" "volumetemplate"
test_resource "images" "image"

# Test nested resources (sandbox processes and job executions)
# These require resources to exist - dynamically get the first one from the list
echo ""
echo "========================================="
echo "Testing nested resources"
echo "========================================="

# Test sandbox processes - get first DEPLOYED sandbox from list
echo "Getting first DEPLOYED sandbox from list..."
SANDBOX_LIST=$(go run main.go get sandboxes -o json 2>&1)
if [ $? -eq 0 ]; then
    SANDBOX_NAME=$(echo "$SANDBOX_LIST" | jq -r '[.[] | select(.status == "DEPLOYED")] | .[0].metadata.name // empty' 2>/dev/null)
    if [ -n "$SANDBOX_NAME" ]; then
        echo "Found deployed sandbox: $SANDBOX_NAME"
        test_sandbox_processes "$SANDBOX_NAME"
    else
        echo "⚠️  No deployed sandboxes found, skipping sandbox process tests"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    fi
else
    echo "⚠️  Failed to list sandboxes, skipping sandbox process tests"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
fi

# Test job executions - get first DEPLOYED job from list
echo ""
echo "Getting first DEPLOYED job from list..."
JOB_LIST=$(go run main.go get jobs -o json 2>&1)
if [ $? -eq 0 ]; then
    JOB_NAME=$(echo "$JOB_LIST" | jq -r '[.[] | select(.status == "DEPLOYED")] | .[0].metadata.name // empty' 2>/dev/null)
    if [ -n "$JOB_NAME" ]; then
        echo "Found deployed job: $JOB_NAME"
        test_job_executions "$JOB_NAME"
    else
        echo "⚠️  No deployed jobs found, skipping job execution tests"
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    fi
else
    echo "⚠️  Failed to list jobs, skipping job execution tests"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
fi

echo ""
echo "========================================="
echo "Test Summary"
echo "========================================="
echo "Total resources tested: $TOTAL_TESTS"
echo "✅ Passed: $PASSED_TESTS"
echo "❌ Failed: $FAILED_TESTS"
echo "⚠️  Skipped: $SKIPPED_TESTS"
echo ""

if [ $FAILED_TESTS -eq 0 ] && [ $PASSED_TESTS -gt 0 ]; then
    echo "🎉 All accessible tests passed!"
    exit 0
elif [ $PASSED_TESTS -eq 0 ] && [ $SKIPPED_TESTS -eq $TOTAL_TESTS ]; then
    echo "⚠️  All tests skipped (likely authentication required)"
    exit 0
else
    echo "Some tests failed. Please review the output above."
    exit 1
fi