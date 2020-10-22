# docforge

Docforge is a command-line tool that builds documentation bundles from markdown sources distributed across multiple repositories.

## Quick start

1. Get a GitHub personal token ([GitHub Account > Settings > Developer Settings > Personal access tokens](https://github.com/settings/tokens))
2. Get a docforge [release](https://github.com/gardener/docforge/releases)
3. Run with a [sample manifest](example/simple/00.yaml):
   ```
   docforge -d tmp/docforge-docs -f example/simple/00.yaml --github-oauth-token <YOUR_GITHUB_TOKEN_HERE> --resources-download-path=__resources
   ```

For options for using docforge see `docforge -h`.

For details on manifests, see the comments in [pkg/api.types.go](pkg/api/types.go).

For details on concepts, such as preserving links consistency, downloads scope and material versions see [documentation](docs).
