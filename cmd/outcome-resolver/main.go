package main

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ajitpratap0/cryptofunk/internal/db"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/gamma"
	"github.com/ajitpratap0/cryptofunk/internal/polymarket/resolver"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Msg("Starting outcome resolver")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	database, err := db.New(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	gammaClient := gamma.NewClient()
	res := resolver.NewResolver(database, gammaClient)

	polyResolved, binanceResolved, err := res.ResolveAll(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Resolution completed with errors")
	}

	log.Info().
		Int("polymarket_resolved", polyResolved).
		Int("binance_resolved", binanceResolved).
		Msg("Outcome resolution complete")
}
