# docforge

Docforge is a command-line tool that builds documentation bundles from markdown sources distributed across multiple repositories, using manifest for a desired structure and links management.

## Quick start

1. Get a GitHub personal token ([GitHub Account > Settings > Developer Settings > Personal access tokens](https://github.com/settings/tokens))
2. Get a docforge [release](https://github.com/gardener/docforge/releases)
3. Run with a [sample manifest](example/simple/00.yaml), specifiying:
   - destination for the documentation bundle (`-d tmp/docforge-docs`)
   - the manifest ot use (`-f example/simple/00.yaml`)
   - the GitHub token to use to pull material (`--github-oauth-token <YOUR_GITHUB_TOKEN_HERE>`)
   
```
docforge -d /tmp/docforge-docs -f example/simple/00.yaml --github-oauth-token <YOUR_GITHUB_TOKEN_HERE> 
```
To see an overview of how `docforge` makes changes to the links in the pulled documents to keep them valid in the intended structure in the manifest, run it with the `--dry-run` option.

For options for using docforge see `docforge -h`.

For details on manifests, see the comments in [pkg/api.types.go](pkg/api/types.go).

For details on concepts, such as preserving links consistency, downloads scope and material versions see [documentation](docs).
