package configs

import (
	"log"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

// Global koanf instance. Use . as the key path delimiter. This can be / or anything.
var (
	k      = koanf.New(".")
	parser = yaml.Parser()
)

type GridConfig struct {
	NumOfRows int `koanf:"rows"`
	NumOfCols int `koanf:"cols"`
}

type AppConfig struct {
	Grid       GridConfig `koanf:"grid"`
	UpdateFreq int        `koanf:"update_frequency"`
}

func LoadConfig() (*AppConfig, error) {
	// Load yaml config.
	if err := k.Load(file.Provider("configs/conf.yaml"), parser); err != nil {
		log.Fatalf("error loading config: %v", err)
		return nil, err
	}
	var out AppConfig

	// Quick unmarshal.
	k.Unmarshal("", &out)

	return &out, nil
}
