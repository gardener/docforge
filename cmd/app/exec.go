package app

import (
	"context"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
)

func exec(ctx context.Context) error {
	var (
		doc     *api.Documentation
		rhs     []resourcehandlers.ResourceHandler
		err     error
		options options
	)

	err = vip.Unmarshal(&options)
	if err != nil {
		return err
	}
	if rhs, err = initResourceHandlers(ctx, options.ResourceHandlerOptions, options.ParsingOptions); err != nil {
		return err
	}
	if doc, err = constructInitialManifest(ctx, options.ManifestPath, rhs, options.ParsingOptions); err != nil {
		return err
	}
	reactor, err := newReactor(options.Options, options.Hugo, rhs)
	if err != nil {
		return err
	}
	if err = reactor.Run(ctx, doc); err != nil {
		return err
	}
	return nil
}
