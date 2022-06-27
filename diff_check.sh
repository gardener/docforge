#!/bin/bash

if [[ $1 == "--help" ]]; then
    grep -e '^##' $0 | cut -d ' ' -f 2-
    exit 0
fi

deleteContent="yes"
if [[ "$1" == "-c" ]]; then
    echo "Branch content will not be deleted"
    deleteContent="no"
    branchName=$2
    gardenerRemote=$3
    websitePath=$4
else 
    branchName=$1
    gardenerRemote=$2
    websitePath=$3
fi
# check if branch exists
if [[ -z $(git branch | grep -e "[[:space:]]${branchName}$") ]]; then
    echo "Branch does not exist"
    exit 1
fi

if [[ -z $(git remote | grep -x "${gardenerRemote}") ]]; then
    echo "Remote does not exist"
    exit 1
fi

if [[ ! -d "${websitePath}/hugo" ]]; then
    echo "Website path does not exits or does not contain hugo folder"
    exit 1
fi

# get current branch
currentBranch=$(git rev-parse --abbrev-ref HEAD)
# building branch docforge
git checkout $branchName
make build-local
mv bin/docforge /usr/local/bin
# building branch content
cd $websitePath
make build
# deleting old branchContent
rm -rf hugo/branchContent
mv hugo/content hugo/branchContent
# fetch latest origin/master
cd -
git fetch --all
# building master docforge
git checkout "${gardenerRemote}/master"
make build-local
mv bin/docforge /usr/local/bin
# building master content
cd $websitePath
make build
# comparing contents
echo "-------------------------------"
echo "Diff results"
diff -r hugo/content hugo/branchContent
echo "-------------------------------"
# dele branch content if needed
if [[ "$deleteContent" == "yes" ]]; then
    rm -rf hugo/branchContent
fi
# return to current branch
cd -
git checkout "$currentBranch"

## A tool for MacOS/Linux(not tested on linux) that compares the differences between generated content of a local docforge PR branch and the master branch:
## 
## 
##        ./diff_check.sh [-c] <local_PR_branch> <remote_name_of_gardener_repo> <path_to_website_generator> 
## 
## 
## With following parameters:
## 
##   -c
##        Optional flag. If present hugo/branchContent directory will not be deleted from website generator folder after tool execution. If flag is omitted the directory will be deleted
## 
##   local_PR_branch
##        The branch name of the PR
## 
##   remote_name_of_gardener_repo
##        Depending from where you have cloned docforge (gardener or your github fork) this most like will be 'origin' or 'upstream' 
## 
##   path_to_website_generator
##        Local path of website generator where content will be generated from 'make build'. Note that the path should contain a 'hugo' folder with a 'content' subfolder and have a 'make build' command defined
## 
## Note: after tool execution docforge binary in /usr/local/bin will be overwritten with the binary of the gardener master branch
## 