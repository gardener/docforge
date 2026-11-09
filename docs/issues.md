## Troubleshooting

### Issue: "no resource handlers were loaded"

**Cause**: Missing GitHub OAuth token configuration

**Solution**: Ensure you have:

1. Created a GitHub personal access token
1. Set it as an environment variable
1. Referenced it in the `--github-oauth-env-map` flag

### Issue: Build fails on Windows with Makefile

**Cause**: Makefile uses Unix-style bash commands

**Solution**: Use the direct `go build` command instead of `make`

- Command: `go build -v -o bin/docforge.exe -ldflags "-w -X github.com/gardener/docforge/cmd/version.Version=v0.57.0-dev" ./cmd`

### Issue: Command not found (Windows)

**Cause**: Trying to run bash scripts on Windows

**Solution**: Either:

- Install Git Bash or WSL
- Use direct Go commands
