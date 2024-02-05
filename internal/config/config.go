package config

import (
	"flag"
	"os"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/notnull-co/cfg"
	"github.com/rs/zerolog"
)

type Configuration struct {
	Rest struct {
		Port string `cfg:"port"`
	} `cfg:"rest"`
	Database struct {
		Schema string `cfg:"schema"`
		Path   string `cfg:"path"`
	} `cfg:"database"`
	Kubernetes struct {
		Config string `cfg:"config"`
	} `cfg:"kubernetes"`
	Logger struct {
		Json bool `cfg:"json"`
	} `cfg:"logger"`
}

var (
	instance Configuration
	once     sync.Once
	initErr  error
)

func Init() error {
	once.Do(func() {
		var configDir string
		flag.StringVar(&configDir, "config-dir", "config/", "Configuration file directory")
		flag.Parse()
		initErr = cfg.Load(&instance, cfg.Dirs(configDir), cfg.UseEnv("cfg"))

		if !instance.Logger.Json {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		}
	})
	return initErr
}

func Get() Configuration {
	return instance
}
