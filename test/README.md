# e2e

The main purpose of e2e tests is to ensure the docforge executable artefact covers the most valuable use case scenarios. Main benefits:
- acceptance criteria when performing refactoring
- could be run in any os environment (local/pipeline)
- could be used as a pre-commit hook or in git bisect

# integration

The main purpose of integration tests is to provide an interactive code execution environment that covers main run configurations. Main benefits:
- useful when performing refactoring
- useful for new contributors to step into code to get a better understanding of the execution flow
- useful when fixing a bug by modifying the run configuration to match the configuration causing the bug if one can obtain it
