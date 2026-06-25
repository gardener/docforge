## 1. Remove link validator package

- [ ] 1.1 Delete `pkg/nodeplugins/markdown/linkvalidator/` directory (validator.go, job.go, validator_test.go, linkvalidatorfakes/)

## 2. Remove validator from markdown node plugin

- [ ] 2.1 Remove validator parameters from `pkg/nodeplugins/markdown/plugin.go` NewPlugin() — drop `skipLinkValidation`, `validationWorkersCount`, `hostsToReport` params and validator creation
- [ ] 2.2 Remove validator task queue from plugin return values
- [ ] 2.3 Remove `linkvalidator` import from plugin.go

## 3. Remove validator from document worker

- [ ] 3.1 Remove `validator` field and `skipLinkValidation` field from Worker struct in `pkg/nodeplugins/markdown/document/document_worker.go`
- [ ] 3.2 Remove `ValidateLink()` call from `resolveLink()` — absolute URLs not in repository should just `return dest, nil` without validation
- [ ] 3.3 Remove validator parameter from `NewDocumentWorker()` constructor
- [ ] 3.4 Remove validator parameter from `New()` in `pkg/nodeplugins/markdown/document/job.go`
- [ ] 3.5 Update document worker tests to remove fake validator usage

## 4. Remove SkipValidation from manifest

- [ ] 4.1 Remove `SkipValidation` field from Node struct in `pkg/manifest/node.go`
- [ ] 4.2 Remove `propagateSkipValidation()` transformation from `pkg/manifestplugins/markdown/plugin.go`

## 5. Remove CLI flags and config

- [ ] 5.1 Remove `--validation-workers`, `--skip-link-validation`, `--hosts-to-report` flags from `cmd/app/flags.go`
- [ ] 5.2 Remove corresponding fields from Options struct in `cmd/app/types.go`
- [ ] 5.3 Remove validator-related parameters from `markdown.NewPlugin()` call in `cmd/app/exec.go`
- [ ] 5.4 Remove `skip-link-validation: true` from `test/e2e/docforge_config.yaml`
- [ ] 5.5 Update `docs/cmd-ref/docforge.md` to remove `--validation-workers` documentation

## 6. Build and verify

- [ ] 6.1 Run `go build ./...` to confirm clean compilation
- [ ] 6.2 Run `go test ./...` to confirm all tests pass
- [ ] 6.3 Clone https://github.com/gardener/documentation locally and delete `hugo/` if it exists
- [ ] 6.4 Build the docforge binary **before** this change and run it against the gardener/documentation repo to produce a baseline `hugo/` output; SHA-256 hash the directory contents, then delete `hugo/`
- [ ] 6.5 Build the docforge binary **after** this change and run it against the same repo; SHA-256 hash the resulting `hugo/` directory, then delete `hugo/`
- [ ] 6.6 Compare the two hashes — they MUST be identical to confirm zero output difference
