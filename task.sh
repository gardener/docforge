#!/bin/bash

set -e

if [[ -z "$1" ]]; then
    echo "Please provide a commit message"
    exit 1
fi
created_files=$(git ls-files --others --exclude-standard)
modified_files=$(git diff --name-only)
staged_files=$(git diff --cached --name-only)
echo -e "-------------"
if [[ -n "${created_files}${modified_files}" ]]; then 
    echo -e "\033[1;31mChanges:\033[0m"
    echo -e "${created_files}${created_files:+\n}${modified_files}"
    echo 
fi
if [[ -n "$staged_files" ]]; then
    echo -e "\033[1;32mStaged changes:\033[0m"
    echo -e "$staged_files"
    echo
fi
echo -e "-------------"
echo 
if [[ -n "${created_files}${modified_files}" ]]; then
    echo "Please stage all files before running this command"
    exit 1
fi
if [[ -z "$staged_files" ]]; then
    echo "No files are staged"
    exit 1
fi

echo "Running tests:"
.ci/test > /dev/null 2>&1
echo "Running integration tests:"
.ci/integration-test > /dev/null 2>&1

commit_message="[task] $1"
git commit -m"$commit_message"
