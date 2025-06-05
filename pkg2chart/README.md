# pkg2chart

A simple utility to convert ArtifactHub package descriptions
(`artifacthub-pkg.yml`) to Helm Charts files (`Chart.yaml`, `values.yaml`) containing
information needed by the Rancher UI to catalog policies.

Note: this tool does not generate a full Helm Chart, only the metadata file.

## Usage

```bash
pkg2chart [--input artifacthub-pkg.yml] [--output .]
```

### Options

- `--input` - Path to the ArtifactHub package description file (default: `artifacthub-pkg.yml`)
- `--output` - Directory to output the Helm Chart files (default: `chart`)
