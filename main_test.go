package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringArrayToMap(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		input    []string
		expected map[string]string
		err      bool
	}{
		{[]string{"key1=value1", "key2=value2"}, map[string]string{"key1": "value1", "key2": "value2"}, false},
		{[]string{"key1=value1", "key2="}, nil, true},
		{[]string{"key1=value1", "=value2"}, nil, true},
		{[]string{"key1=value1", "key2value2"}, nil, true},
	}

	for _, tt := range tests {
		result, err := stringArrayToMap(tt.input)
		if tt.err {
			assert.Error(err)
		} else {
			assert.NoError(err)
			assert.Equal(tt.expected, result)
		}
	}
}

func TestPopulateParameterMap(t *testing.T) {
	assert := assert.New(t)

	testJson := "{\"value3\": {\"foo\": \"bar\"}}"
	testObj := make(map[string]interface{})
	err := json.Unmarshal([]byte(testJson), &testObj)
	assert.NoError(err)

	testFile := filepath.Join(t.TempDir(), "test.json")
	assert.NoError(os.WriteFile(testFile, []byte(testJson), 0600))

	tests := []struct {
		input    Options
		expected map[string]interface{}
		err      bool
	}{
		{
			Options{
				Values:     []string{"key1=value1", "key2=value2"},
				ValueFiles: []string{"key3=" + testFile},
			},
			map[string]interface{}{
				"Values": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
					"key3": testObj,
				},
			},
			false,
		},
		{
			Options{
				Values:     []string{"key1=value1", "key2=value2"},
				ValueFiles: []string{"key3=does-not-exist.json"},
			},
			map[string]interface{}{
				"Values": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
					"key3": testObj,
				},
			},
			true,
		},
	}

	for _, tt := range tests {
		result, err := populateParameterMap(tt.input)
		if tt.err {
			assert.Error(err)
		} else {
			assert.NoError(err)
			assert.Equal(tt.expected, result)
		}
	}
}
