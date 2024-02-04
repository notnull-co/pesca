package config

import (
	"flag"
	"sync"

	"github.com/notnull-co/cfg"
)

type Configuration struct {
}

var (
	instance *Configuration
	once     sync.Once
	initErr  error
)

func Init() error {
	once.Do(func() {
		var configDir string
		flag.StringVar(&configDir, "config-dir", "config/", "Configuration file directory")
		flag.Parse()
		initErr = cfg.Load(instance, cfg.Dirs(configDir))
	})
	return initErr
}

func Get() Configuration {
	return *instance
}
