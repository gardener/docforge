package app

import (
	"context"

	"github.com/gardener/docforge/pkg/resourcehandlers"
	"k8s.io/klog/v2"
)

func exec(ctx context.Context) error {
	var (
		rhs     []resourcehandlers.ResourceHandler
		err     error
		options options
	)

	err = vip.Unmarshal(&options)
	klog.Infof("Manifest: %s", options.ManifestPath)
	for resource, mapped := range options.ResourceMappings {
		klog.Infof("%s -> %s", resource, mapped)
	}
	klog.Infof("Output dir: %s", options.DestinationPath)
	if err != nil {
		return err
	}
	if rhs, err = initResourceHandlers(ctx, options.ResourceHandlerOptions, options.ParsingOptions); err != nil {
		return err
	}
	reactor, err := newReactor(options.Options, options.Hugo, rhs)
	if err != nil {
		return err
	}
	if err = reactor.Run(ctx, options.ManifestPath); err != nil {
		return err
	}
	return nil
}
