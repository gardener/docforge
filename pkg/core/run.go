package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
)

// Run is the method that constructs the website bundle
func Run(ctx context.Context, nodes []*manifest.Node, reactorWG *sync.WaitGroup, plugins []nodeplugins.Interface, pluginQC []taskqueue.QueueController) error {
	qcc := taskqueue.NewQueueControllerCollection(reactorWG, pluginQC...)

	processorToPlugin := map[string]nodeplugins.Interface{}
	for _, plugin := range plugins {
		processorToPlugin[plugin.Processor()] = plugin

	}
	for _, node := range nodes {
		if node.Type != "file" {
			continue
		}
		if processor, ok := processorToPlugin[node.Processor]; ok {
			if err := processor.Process(node); err != nil {
				return fmt.Errorf("processor %s failed processing node \n%s\n: %w", processor.Processor(), node, err)
			}
		} else {
			// TODO may be undesired if we expect multiple core.Run calls
			return fmt.Errorf("node \n%s\n did not have a processor", node)
		}
	}

	qcc.Start(ctx)
	qcc.Wait()
	qcc.Stop()
	qcc.LogTaskProcessed()
	return qcc.GetErrorList().ErrorOrNil()
}
