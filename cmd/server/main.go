package main

import (
	"github.com/notnull-co/pesca/internal/channel/rest"
	"github.com/notnull-co/pesca/internal/config"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	log.Fatal().Err(rest.New().Start()).Msg("rest server closed unexpectedly")
}
