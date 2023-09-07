package app

import (
	"context"

	"github.com/gardener/docforge/pkg/resourcehandlers"
)

func exec(ctx context.Context) error {
	var (
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
	reactor, err := newReactor(options.Options, options.Hugo, rhs)
	if err != nil {
		return err
	}
	if err = reactor.Run(ctx, options.ManifestPath); err != nil {
		return err
	}
	return nil
}
