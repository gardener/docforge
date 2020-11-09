# docforge

[![reuse compliant](https://reuse.software/badge/reuse-compliant.svg)](https://reuse.software/)

Docforge is a command-line tool that builds documentation bundles from markdown sources distributed across multiple repositories, using manifest for a desired structure and links management.

## Installation

### Users

Go to project [releases](https://github.com/gardener/docforge/releases), select the [latest release](https://github.com/gardener/docforge/releases/latest) and pickup a download for your OS:

- **Mac**: docforge-darwin-amd64
- **Linux**: docforge-linux-amd64
- **Windows**: docforge-windows-386.exe

> **Disclaimer on releases**: Until there is a stable 1.0 version changes are likely to occur and not necessarily backwards compatible. New features are released with a minor version increase. We do not release hotfixes except for the latest minor release, only for bugs and only when critical.

For convenience, copy the docforge executable somewhere on your PATH. For Linux/Mac `/usr/local/bin` is usually a good spot.

### Developers

``` sh
go get github.com/gardener/docforge
```

### Operators

Docker images with all docforge releases are public at [Google Cloud Registry](https://console.cloud.google.com/gcr/images/gardener-project/EU/docforge?project=gardener-project&gcrImageListsize=30). To pull a docforge image for a release use the release as image tag, e.g. for docforge version [`v0.5.1`](https://github.com/gardener/docforge/releases/tag/v0.5.1):
```sh
docker pull eu.gcr.io/gardener-project/docforge:v0.5.1
```

## Usage

> **GitHub API Disclaimer**: The docforge tool can pull material from GitHub and it will use the GitHub API for that. The API has certain usage [rate limits](https://docs.github.com/en/free-pro-team@latest/rest/overview/resources-in-the-rest-api#rate-limiting) and they are very easy to hit for unauthenticated requests - up to 60 requests per hour per originating IP as of the time of this writing. It is highly recommended to create a GitHub personal token ([GitHub Account > Settings > Developer Settings > Personal access tokens](https://github.com/settings/tokens)) and supply it to `docforge` with the `--github-oauth-token` flag.


### Basics

- **Getting help**
   ```sh
   dockforge -h
   ```
- **Getting docforge version**
   ```sh
   dockforge version
   ```

### Forge a build

To create a documentation material bundle, it is necessary to describe it in a manifest file. Example manifests can be found in the [example](example) directory of this project. For more information on creating manifests, see the [User guide](TODO).

Assuming that:
- the **destination** where the forged bundle will appear is `/tmp/docforge-docs`, and 
- the **manifest** file is [example/simple/00.yaml](example/simple/00.yaml), and
- the **GitHub token** to use when reading documents from GitHub specified in the manifest is stored in a `$GITHUB_TOKEN` environment variable,
the command to forge the bundle is:
```sh
docforge -d /tmp/docforge-docs -f example/simple/00.yaml --github-oauth-token $GITHUB_TOKEN
```

### Analyze

- **Links conversions, total build time**   
   To print an overview of the changes that docforge does to the links in each pulled document to keep them valid in the intended structure in the manifest, run it with the `--dry-run` option. This will forge a full build, but without serializing the structure, and will provide you with the insight of changes per document.

- **Resolved structure**   
   The docforge manifests support many implicit ways to specify a structure by rules, such as inclusion/exclusion patterns. To print the actual structure to which those constructs will resolve use the `--resolve` flag.

- **All in one**   
   The `--dry-run` and `--resolve` flags can be combined for a full analytic overview.

## What's next
- [ User Documentation](docs/user-index.md)