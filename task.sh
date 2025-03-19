#!/usr/bin/env bash

set -e

user_message=$1
if [[ -z "$user_message" ]]; then
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
echo
echo "Commiting task-changes-for-e2e on current branch."
echo -e "\033[1;33mNote:\033[0m Revert commit using git reset --soft HEAD~1 if e2e fails"
echo
git commit -m"task-changes-for-e2e"
echo "Running e2e tests:" 
test/e2e/diff.sh  > /dev/null 2>&1
echo "Reverting task-changes-for-e2e from current branch:"
git reset --soft HEAD~1

longest_common_prefix() {
    prefix=$1
    for path in "$@"; do
        while [[ "$path" != "$prefix"* ]]; do
            if [[ $prefix == ${prefix%/*} ]]; then
                echo "[tested] "
                return
            fi
            prefix=${prefix%/*}
        done
    done

    echo "${prefix}: "
}

prefix=$(longest_common_prefix $(echo "$staged_files" | tr '\n' ' '))
commit_message="${prefix}${user_message}"
git commit -m"$commit_message"
