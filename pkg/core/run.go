package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"k8s.io/klog/v2"
)

// Run is the method that constructs the website bundle
func Run(ctx context.Context, nodes []*manifest.Node, reactorWG *sync.WaitGroup, plugins []nodeplugins.Interface, pluginQC []taskqueue.QueueController) error {
	qcc := taskqueue.NewQueueControllerCollection(reactorWG, pluginQC...)

	processorToPlugin := map[string]nodeplugins.Interface{}
	for _, plugin := range plugins {
		processorToPlugin[plugin.Processor()] = plugin

	}

	// Collect all channels from ProcessNew calls
	var allChannels []chan nodeplugins.Status

	for _, node := range nodes {
		if node.Type != "file" {
			continue
		}
		if processor, ok := processorToPlugin[node.Processor]; ok {
			if node.Processor == "downloader" || node.Processor == "markdown" {
				channels := processor.ProcessNew(node)
				allChannels = append(allChannels, channels...)
			} else {
				if err := processor.Process(node); err != nil {
					return fmt.Errorf("processor %s failed processing node \n%s\n: %w", processor.Processor(), node, err)
				}
			}
		} else {
			// TODO may be undesired if we expect multiple core.Run calls
			return fmt.Errorf("node \n%s\n did not have a processor", node)
		}
	}

	// Wait for all channels to complete and collect external links
	var allExternalLinks []manifest.ExternalLink
	for _, ch := range allChannels {
		status := <-ch
		if status.Error() != nil {
			return fmt.Errorf("channel processing failed: %w", status.Error())
		}

		// Collect external links from this processing result
		if links := status.ExternalLinks(); links != nil {
			allExternalLinks = append(allExternalLinks, links...)
		}
	}

	// Log collected external links for verification
	logCollectedLinksFromStatus(allExternalLinks)

	qcc.Start(ctx)
	qcc.Wait()
	qcc.Stop()
	qcc.LogTaskProcessed()
	return qcc.GetErrorList().ErrorOrNil()
}

func logCollectedLinksFromStatus(allLinks []manifest.ExternalLink) {
	totalLinks := len(allLinks)
	uniqueLinks := make(map[string]int)

	for _, link := range allLinks {
		uniqueLinks[link.URL]++
	}

	if totalLinks > 0 {
		klog.Infof("Collected %d external links (%d unique) for potential deferred validation",
			totalLinks, len(uniqueLinks))

		// Show top duplicated links to demonstrate deduplication potential
		for url, count := range uniqueLinks {
			if count > 1 {
				klog.Infof("Link %s referenced %d times", url, count)
			}
		}
	}
}
