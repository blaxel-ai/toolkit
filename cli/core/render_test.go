package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncate with ellipsis", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"max length 0", "hello", 0, ""},
		{"empty string", "", 10, ""},
		{"max 4 with ellipsis", "hello world", 4, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLength)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatVolumeSize(t *testing.T) {
	tests := []struct {
		name     string
		sizeInMB int
		expected string
	}{
		{"less than 1GB", 512, "512 MB"},
		{"exactly 1GB", 1024, "1.00 GB"},
		{"more than 1GB", 2048, "2.00 GB"},
		{"1.5GB", 1536, "1.50 GB"},
		{"zero", 0, "0 MB"},
		{"large size", 10240, "10.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatVolumeSize(tt.sizeInMB)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytesSize(t *testing.T) {
	tests := []struct {
		name        string
		sizeInBytes int64
		expected    string
	}{
		{"bytes", 512, "512 B"},
		{"kilobytes", 2048, "2.00 KB"},
		{"megabytes", 1048576, "1.00 MB"},
		{"gigabytes", 1073741824, "1.00 GB"},
		{"terabytes", 1099511627776, "1.00 TB"},
		{"zero", 0, "0 B"},
		{"1.5 GB", 1610612736, "1.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytesSize(tt.sizeInBytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"int", 1024, "1.00 GB"},
		{"float64", 2048.0, "2.00 GB"},
		{"string number", "512", "512 MB"},
		{"string text", "unknown", "unknown"},
		{"nil", nil, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSizeValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatImageSizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"int", 1073741824, "1.00 GB"},
		{"int64", int64(1073741824), "1.00 GB"},
		{"float64", 1073741824.0, "1.00 GB"},
		{"string number", "1048576", "1.00 MB"},
		{"string text", "unknown", "unknown"},
		{"nil", nil, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatImageSizeValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNavigateToKey(t *testing.T) {
	testMap := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "deep value",
			},
			"simple": "simple value",
		},
		"top": "top value",
	}

	tests := []struct {
		name     string
		keys     []string
		expected interface{}
	}{
		{"single level", []string{"top"}, "top value"},
		{"two levels", []string{"level1", "simple"}, "simple value"},
		{"three levels", []string{"level1", "level2", "value"}, "deep value"},
		{"non-existent key", []string{"nonexistent"}, nil},
		{"empty keys", []string{}, nil},
		{"partial path", []string{"level1", "nonexistent"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := navigateToKey(testMap, tt.keys)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetrieveKey(t *testing.T) {
	testMap := map[string]interface{}{
		"status": "DEPLOYED",
		"metadata": map[string]interface{}{
			"name":      "test-agent",
			"workspace": "my-workspace",
		},
		"spec": map[string]interface{}{
			"runtime": map[string]interface{}{
				"image":  "my-image:latest",
				"memory": 4096,
			},
		},
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"top level key", "status", "DEPLOYED"},
		{"metadata name", "name", "test-agent"},
		{"nested spec.runtime.image", "spec.runtime.image", "my-image:latest"},
		{"non-existent key", "nonexistent", "-"},
		{"metadata workspace", "workspace", "my-workspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retrieveKey(testMap, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		format    string
		expected  string
	}{
		{"empty timestamp", "", "2006-01-02", "-"},
		{"dash timestamp", "-", "2006-01-02", "-"},
		{"invalid timestamp", "invalid", "2006-01-02", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDate(tt.timestamp, tt.format)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test with valid timestamp (local time dependent)
	t.Run("valid RFC3339 timestamp", func(t *testing.T) {
		timestamp := "2024-01-15T10:30:00Z"
		result := formatDate(timestamp, "2006-01-02")
		// The result depends on local timezone, but should be a valid date
		_, err := time.Parse("2006-01-02", result)
		assert.NoError(t, err)
	})

	t.Run("valid RFC3339Nano timestamp", func(t *testing.T) {
		timestamp := "2024-01-15T10:30:00.123456789Z"
		result := formatDate(timestamp, "2006-01-02 15:04:05")
		// The result depends on local timezone, but should be a valid datetime
		_, err := time.Parse("2006-01-02 15:04:05", result)
		assert.NoError(t, err)
	})
}

func TestSortByCreationDate(t *testing.T) {
	slices := []interface{}{
		map[string]interface{}{
			"metadata": map[string]interface{}{"name": "oldest"},
			"createdAt": "2024-01-01T00:00:00Z",
		},
		map[string]interface{}{
			"metadata": map[string]interface{}{"name": "newest"},
			"createdAt": "2024-01-03T00:00:00Z",
		},
		map[string]interface{}{
			"metadata": map[string]interface{}{"name": "middle"},
			"createdAt": "2024-01-02T00:00:00Z",
		},
	}

	sorted := sortByCreationDate(slices)

	// Should be sorted newest first
	assert.Equal(t, "newest", sorted[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
	assert.Equal(t, "middle", sorted[1].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
	assert.Equal(t, "oldest", sorted[2].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
}

func TestSortByCreationDateWithInvalidDates(t *testing.T) {
	slices := []interface{}{
		map[string]interface{}{
			"metadata": map[string]interface{}{"name": "valid"},
			"createdAt": "2024-01-01T00:00:00Z",
		},
		map[string]interface{}{
			"metadata": map[string]interface{}{"name": "invalid"},
			"createdAt": "invalid-date",
		},
	}

	// Should not panic with invalid dates
	sorted := sortByCreationDate(slices)
	assert.Len(t, sorted, 2)
}

func TestBuildTableHeader(t *testing.T) {
	resource := Resource{
		Kind: "Agent",
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "STATUS", Value: "status"},
		},
	}

	header := buildTableHeader(resource)
	assert.Len(t, header, 3)
	assert.Equal(t, "WORKSPACE", header[0])
	assert.Equal(t, "NAME", header[1])
	assert.Equal(t, "STATUS", header[2])
}

func TestRetrieveFieldValue(t *testing.T) {
	itemMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-agent",
		},
		"status": "DEPLOYED",
		"spec": map[string]interface{}{
			"runtime": map[string]interface{}{
				"image":  "sandbox/my-image:latest",
				"memory": float64(4096),
			},
			"size": float64(1024),
		},
		"createdAt": "2024-01-15T10:30:00Z",
		"items": []interface{}{"a", "b", "c"},
	}

	tests := []struct {
		name     string
		field    Field
		expected string
	}{
		{"simple field", Field{Key: "STATUS", Value: "status"}, "DEPLOYED"},
		{"count field", Field{Key: "COUNT", Value: "items", Special: "count"}, "3"},
		{"size field", Field{Key: "SIZE", Value: "spec.size", Special: "size"}, "1.00 GB"},
		{"image field with prefix", Field{Key: "IMAGE", Value: "spec.runtime.image", Special: "image"}, "my-image:latest"},
		{"date field", Field{Key: "CREATED_AT", Value: "createdAt", Special: "date"}, formatDate("2024-01-15T10:30:00Z", "2006-01-02")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retrieveFieldValue(itemMap, tt.field, 100)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildTableRow(t *testing.T) {
	resource := Resource{
		Kind: "Agent",
		Fields: []Field{
			{Key: "NAME", Value: "metadata.name"},
			{Key: "STATUS", Value: "status"},
		},
	}

	itemMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test-agent",
		},
		"status": "DEPLOYED",
	}

	row := buildTableRow(resource, itemMap, 100)
	assert.Len(t, row, 2)
	assert.Equal(t, "test-agent", row[0])
	assert.Equal(t, "DEPLOYED", row[1])
}

func TestBuildTableRowForImage(t *testing.T) {
	resource := Resource{
		Kind: "Image",
		Fields: []Field{
			{Key: "NAME", Value: "metadata.name"},
		},
	}

	itemMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":         "test-image",
			"resourceType": "agent",
		},
	}

	row := buildTableRow(resource, itemMap, 100)
	assert.Len(t, row, 1)
	assert.Equal(t, "agent/test-image", row[0])
}

func TestRetrieveFieldValueWithDatetime(t *testing.T) {
	itemMap := map[string]interface{}{
		"updatedAt": "2024-01-15T10:30:45Z",
	}

	field := Field{Key: "UPDATED_AT", Value: "updatedAt", Special: "datetime"}
	result := retrieveFieldValue(itemMap, field, 100)

	// Result should be formatted as datetime
	_, err := time.Parse("2006-01-02 15:04:05", result)
	assert.NoError(t, err)
}

func TestRetrieveFieldValueWithImageSize(t *testing.T) {
	itemMap := map[string]interface{}{
		"size": int64(1073741824), // 1 GB in bytes
	}

	field := Field{Key: "SIZE", Value: "size", Special: "imagesize"}
	result := retrieveFieldValue(itemMap, field, 100)
	assert.Equal(t, "1.00 GB", result)
}

func TestRetrieveFieldValueMissing(t *testing.T) {
	itemMap := map[string]interface{}{}

	field := Field{Key: "SIZE", Value: "size", Special: "size"}
	result := retrieveFieldValue(itemMap, field, 100)
	assert.Equal(t, "-", result)

	field2 := Field{Key: "SIZE", Value: "size", Special: "imagesize"}
	result2 := retrieveFieldValue(itemMap, field2, 100)
	assert.Equal(t, "-", result2)

	field3 := Field{Key: "COUNT", Value: "items", Special: "count"}
	result3 := retrieveFieldValue(itemMap, field3, 100)
	assert.Equal(t, "0", result3)
}

func TestRetrieveFieldValueImageTruncation(t *testing.T) {
	itemMap := map[string]interface{}{
		"image": "sandbox/very-long-image-name-that-should-be-truncated:latest",
	}

	field := Field{Key: "IMAGE", Value: "image", Special: "image"}
	result := retrieveFieldValue(itemMap, field, 20)
	assert.LessOrEqual(t, len(result), 20)
}
