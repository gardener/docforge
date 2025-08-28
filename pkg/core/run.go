package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/core/linkvalidator"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"k8s.io/klog/v2"
)

// Run is the method that constructs the website bundle
func Run(ctx context.Context, nodes []*manifest.Node, pluginList []Plugin, deferredValidation bool, registry registry.Interface, hostsToReport []string, validationWorkersCount int) error {
	processorToPlugin := map[string]Plugin{}
	for _, plugin := range pluginList {
		processorToPlugin[plugin.Processor()] = plugin

	}

	// Collect all channels from ProcessNew calls
	var allChannels []chan Status

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

	// Perform deduplication for efficiency analysis
	linkMap := deduplicateLinks(allExternalLinks)
	logDeduplicationStats(linkMap)

	// Perform deferred validation if enabled
	if deferredValidation && len(linkMap) > 0 {
		klog.Infof("Deferred link validation enabled - validating %d unique URLs with %d workers",
			len(linkMap), validationWorkersCount)

		if err := validateLinksInParallel(ctx, linkMap, validationWorkersCount, registry, hostsToReport); err != nil {
			return fmt.Errorf("deferred link validation failed: %w", err)
		}

		klog.Infof("Deferred validation completed successfully for all %d URLs", len(linkMap))
	} else if len(linkMap) > 0 {
		klog.V(1).Infof("Deferred validation disabled - using immediate validation for %d unique URLs", len(linkMap))
	}

	return nil
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

// deduplicateLinks groups external links by URL, returning a map of URL to source files
func deduplicateLinks(allLinks []manifest.ExternalLink) map[string][]string {
	linkMap := make(map[string][]string)

	for _, link := range allLinks {
		linkMap[link.URL] = append(linkMap[link.URL], link.SourceFile)
	}

	return linkMap
}

// logDeduplicationStats shows the benefits of link deduplication
func logDeduplicationStats(linkMap map[string][]string) {
	totalLinks := 0
	for _, sources := range linkMap {
		totalLinks += len(sources)
	}

	if len(linkMap) > 0 {
		savedValidations := totalLinks - len(linkMap)
		klog.Infof("Deduplication: %d total links → %d unique URLs (saved %d validations)",
			totalLinks, len(linkMap), savedValidations)

		// Show high-impact links (referenced in multiple files)
		for url, sources := range linkMap {
			if len(sources) > 2 {
				klog.V(1).Infof("High-impact link %s referenced in %d files: %v",
					url, len(sources), sources)
			}
		}
	}
}

// linkValidationJob represents a validation task for a single URL
type linkValidationJob struct {
	URL        string
	SourceFile string
}

// validateLinksInParallel validates links using a worker pool pattern
// workerCount controls HTTP client pooling - each worker maintains its own ValidatorWorker
// with dedicated HTTP clients, which is more efficient than creating a client per URL
func validateLinksInParallel(ctx context.Context, linkMap map[string][]string, workerCount int, registry registry.Interface, hostsToReport []string) error {
	if len(linkMap) == 0 {
		return nil
	}

	// Create channels for job distribution
	jobs := make(chan linkValidationJob, len(linkMap))
	results := make(chan error, len(linkMap))

	// Start workers - each gets its own ValidatorWorker for HTTP client pooling
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each worker gets its own ValidatorWorker with dedicated HTTP clients
			// This provides better resource management and HTTP connection reuse
			validatorWorker, err := linkvalidator.NewValidatorWorker(registry, hostsToReport)
			if err != nil {
				results <- fmt.Errorf("failed to create validator worker: %w", err)
				return
			}

			// Process jobs from the queue
			for job := range jobs {
				err := validatorWorker.Validate(ctx, job.URL, job.SourceFile)
				results <- err
			}
		}()
	}

	// Send all validation jobs
	for url, sourceFiles := range linkMap {
		jobs <- linkValidationJob{
			URL:        url,
			SourceFile: sourceFiles[0], // Use first source file for context
		}
	}
	close(jobs)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect validation results
	var validationErrors []error
	for err := range results {
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	// Report summary
	if len(validationErrors) > 0 {
		klog.Warningf("Deferred validation completed with %d errors out of %d URLs",
			len(validationErrors), len(linkMap))
		// Log first few errors for debugging
		for i, err := range validationErrors {
			if i >= 3 { // Limit error output
				klog.Warningf("... and %d more validation errors", len(validationErrors)-i)
				break
			}
			klog.Warningf("Validation error: %v", err)
		}
		return fmt.Errorf("deferred link validation failed with %d errors", len(validationErrors))
	}

	return nil
}
