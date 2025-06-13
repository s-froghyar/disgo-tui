package configs

import (
	_ "embed"
	"log"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
)

//go:embed conf.yaml
var configYAML []byte

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
	// Load yaml config from embedded bytes.
	if err := k.Load(rawbytes.Provider(configYAML), parser); err != nil {
		log.Fatalf("error loading config: %v", err)
		return nil, err
	}
	var out AppConfig

	// Quick unmarshal.
	k.Unmarshal("", &out)

	return &out, nil
}
