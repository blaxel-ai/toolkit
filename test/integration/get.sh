#!/bin/bash

echo "========================================="
echo "Testing GET command for all resources"
echo "========================================="

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