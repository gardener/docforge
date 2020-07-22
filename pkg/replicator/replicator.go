package replicator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gardener/docode/pkg/reactor"
	"github.com/google/go-github/v32/github"
)

// Description holds desired structure state
type Description struct {
	Source  string
	Path    string
	Version string
	Target  string
}

func Replicate(ctx context.Context, client *github.Client, desc *Description) error {
	ghWorker := &reactor.GitHubWorker{
		Client: client,
	}

	job := &reactor.Job{
		MaxWorkers: 50,
		MinWorkers: 1,
		FailFast:   false,
		Worker:     ghWorker,
	}

	var source = desc.Source
	if strings.HasPrefix(desc.Source, "https://") {
		source = strings.TrimPrefix(desc.Source, "https://")
	} else if strings.HasPrefix(desc.Source, "http://") {
		source = strings.TrimPrefix(desc.Source, "http://")
	}

	sourceComponents := strings.Split(source, "/")
	owner := sourceComponents[1]
	repo := sourceComponents[2]

	gitTree, response, err := client.Git.GetTree(ctx, owner, repo, desc.Version, true)
	if err != nil || response.StatusCode != 200 {
		return fmt.Errorf("not 200 status code returned, but %d. Failed to get Gardener tree: %v ", response.StatusCode, err)
	}

	// get the source tree entry as a starting point. gardener/docs for example
	var sourceEntry *github.TreeEntry
	for _, entry := range gitTree.Entries {
		if entry.GetType() == "tree" && entry.GetPath() == desc.Path {
			sourceEntry = entry
			break
		}
	}

	if sourceEntry == nil {
		return fmt.Errorf("couldn't find such entry")
	}

	sourceTree, response, err := client.Git.GetTree(ctx, owner, repo, *sourceEntry.SHA, true)
	if err != nil || response.StatusCode != 200 {
		return fmt.Errorf("not 200 status code returned, but %d. Failed to get Gardener tree: %v ", response.StatusCode, err)

	}

	parentDir := fmt.Sprintf("%s/%s", desc.Target, sourceEntry.GetPath())
	for _, entry := range sourceTree.Entries {
		if entry.GetType() == "tree" {
			mkdirFromTreeEntry(parentDir, entry)
		}
	}

	tasks := make([]interface{}, 0)
	for _, entry := range sourceTree.Entries {
		if entry.GetType() == "blob" {
			tasks = append(tasks, reactor.NewGitHubTask(parentDir, owner, repo, entry.GetSHA(), entry.GetPath()))
		}
	}

	return job.Dispatch(ctx, tasks)
}

func mkdirFromTreeEntry(parentPath string, entry *github.TreeEntry) (string, error) {
	path := entry.GetPath()
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		dirPath := fmt.Sprintf("%s/%s", parentPath, path)
		err := os.MkdirAll(dirPath, os.ModePerm)
		return dirPath, err
	}
	return path, nil
}
