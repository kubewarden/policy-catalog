package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

const (
	artifactHubBaseURL = "https://artifacthub.io/api/v1"
	orgName            = "kubewarden"
	limit              = 60
)

// ArtifactHubPkg represents the structure of an artifacthub-pkg.yml file.
type ArtifactHubPkg struct {
	Name             string            `yaml:"name"`
	DisplayName      string            `yaml:"displayName"`
	Version          string            `yaml:"version"`
	Description      string            `yaml:"description"`
	Keywords         []string          `yaml:"keywords"`
	HomeURL          string            `yaml:"homeURL"`
	Annotations      map[string]string `yaml:"annotations"`
	ContainersImages []ContainerImage  `yaml:"containersImages"`
}

// ArtifactHubRepo represents the structure of an artifacthub-repo.yml file.
type ArtifactHubRepo struct {
	RepositoryID string `yaml:"repositoryID"`
}

// RepositoryResponse represents the structure of a response from the ArtifactHub API.
type RepositoryResponse struct {
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
}

// ContainerImage represents the structure of a container image in an artifacthub-pkg.yml file.
type ContainerImage struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

func main() {
	pkgPath := flag.String("pkg", "artifacthub-pkg.yml", "Path to the artifacthub-pkg.yml file")
	repoPath := flag.String("repo", "artifacthub-repo.yml", "Path to the artifacthub-repo.yml file")

	outputPath := flag.String("output", "Chart.yaml", "Path to output Chart.yaml file")

	flag.Parse()

	if len(flag.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "Error: unrecognized flag or argument: %v\n", flag.Args())
		os.Exit(1)
	}

	err := pkgToChart(*pkgPath, *repoPath, *outputPath, artifactHubBaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "Successfully converted %s to %s\n", *pkgPath, *outputPath)
}

// pkgToChart converts an artifacthub-pkg.yml file to a Chart.yaml file.
func pkgToChart(pkgPath, repoPath, outputPath, baseURL string) error {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	var pkg ArtifactHubPkg
	err = yaml.Unmarshal(data, &pkg)
	if err != nil {
		return fmt.Errorf("error parsing YAML: %w", err)
	}

	var pkgRepo ArtifactHubRepo
	data, err = os.ReadFile(repoPath)
	if err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}
	err = yaml.Unmarshal(data, &pkgRepo)
	if err != nil {
		return fmt.Errorf("error parsing YAML: %w", err)
	}

	repositoryName, err := getRepositoryNameByID(baseURL, pkgRepo.RepositoryID)
	if err != nil {
		return fmt.Errorf("error getting repository name: %w", err)
	}

	// Delete the `kubewarden/questions-ui` annotation.
	// Rancher retrieves the questions.yaml file directly from the chart directory when listing a catalog.
	annotations := pkg.Annotations
	delete(annotations, "kubewarden/questions-ui")

	annotations["artifacthub.io/repository"] = repositoryName

	if len(pkg.ContainersImages) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no container image found in artifacthub-pkg.yml\n")
		os.Exit(1)
	}

	ociURL := pkg.ContainersImages[0].Image
	ref, err := name.NewTag(ociURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing container image URL: %v\n", err)
		os.Exit(1)
	}

	annotations["kubewarden/registry"] = ref.Context().RegistryStr()
	annotations["kubewarden/repository"] = ref.Context().RepositoryStr()
	annotations["kubewarden/tag"] = ref.TagStr()

	// Add annotations required by Rancher
	annotations["catalog.cattle.io/ui-component"] = "kubewarden"
	annotations["catalog.cattle.io/hidden"] = "true"
	annotations["catalog.cattle.io/type"] = "kubewarden-policy"

	annotations["kubewarden/displayName"] = pkg.DisplayName

	metadata := chart.Metadata{
		Name:        pkg.Name,
		Version:     pkg.Version,
		AppVersion:  pkg.Version,
		Description: pkg.Description,
		Keywords:    pkg.Keywords,
		Home:        pkg.HomeURL,
		Sources:     []string{pkg.ContainersImages[0].Image},
		Annotations: pkg.Annotations,
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		err = os.MkdirAll(outputDir, 0o750)
		if err != nil {
			return fmt.Errorf("error creating output directory: %w", err)
		}
	}

	err = chartutil.SaveChartfile(outputPath, &metadata)
	if err != nil {
		return fmt.Errorf("error saving Chart.yaml: %w", err)
	}

	return nil
}

// getRepositoryNameByID retrieves the name of a repository by its ID from the ArtifactHub API.
func getRepositoryNameByID(baseURL, repoID string) (string, error) {
	offset := 0

	for {
		// Since the ArtifactHub API does not provide a way to search for a repository by ID,
		// we need to retrieve all repositories from the organization and search for the repository by ID.
		// The limit is set to 60 because it is the maximum number of repositories that can be retrieved in a single request.
		repoURL := fmt.Sprintf("%s/repositories/search?org=%s&limit=%d&offset=%d",
			baseURL, orgName, limit, offset)

		url, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}

		resp, err := http.Get(url.String()) //nolint:noctx // it is fine to not use a context here
		if err != nil {
			return "", fmt.Errorf("HTTP request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}

		if err := resp.Body.Close(); err != nil {
			return "", fmt.Errorf("failed to close response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var repositories []RepositoryResponse
		if err := json.Unmarshal(body, &repositories); err != nil {
			return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
		}

		if len(repositories) == 0 {
			break
		}

		for _, repo := range repositories {
			if repo.RepositoryID == repoID {
				return repo.Name, nil
			}
		}

		offset += limit
	}

	return "", fmt.Errorf("repository with ID %s not found", repoID)
}
