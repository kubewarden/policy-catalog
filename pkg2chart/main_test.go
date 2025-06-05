package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPkgToChart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repositories/search" {
			t.Fatalf("Unexpected request path: got %s, want /repositories/search", r.URL.Path)
		}

		query := r.URL.Query()
		if query.Get("org") != "kubewarden" {
			t.Fatalf("Unexpected org parameter: got %s, want kubewarden", query.Get("org"))
		}

		if query.Get("offset") != "0" {
			t.Fatalf("Unexpected offset parameter: got %s, want 0", query.Get("offset"))
		}

		if query.Get("limit") != "60" {
			t.Fatalf("Unexpected limit parameter: got %s, want 60", query.Get("limit"))
		}

		fixtureData, err := os.ReadFile("./test/data/repositories_response.json")
		assert.NoError(t, err, "Failed to load test fixture")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(fixtureData)
		assert.NoError(t, err, "Failed to write response")
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	pkgPath := "./test/data/artifacthub-pkg.yml"
	repoPath := "./test/data/artifacthub-repo.yml"
	outputDir := tmpDir
	chartPath := filepath.Join(outputDir, "Chart.yaml")
	valuesPath := filepath.Join(outputDir, "values.yaml")

	err := pkgToChart(pkgPath, repoPath, outputDir, server.URL)
	require.NoError(t, err)

	_, err = os.Stat(chartPath)
	require.NoError(t, err, "Output Chart.yaml file was not created")

	actual, err := os.ReadFile(chartPath)
	require.NoError(t, err)
	expected, err := os.ReadFile("./test/data/Chart.yaml")
	require.NoError(t, err)
	assert.Equal(t, string(expected), string(actual), "Generated Chart.yaml does not match expected fixture")

	_, err = os.Stat(valuesPath)
	require.NoError(t, err, "Output values.yaml file was not created")

	actualValues, err := os.ReadFile(valuesPath)
	require.NoError(t, err)
	expectedValues, err := os.ReadFile("./test/data/values.yaml")
	require.NoError(t, err)
	assert.Equal(t, string(expectedValues), string(actualValues), "Generated values.yaml does not match expected fixture")
}
