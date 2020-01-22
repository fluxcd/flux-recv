package main

import (
	"fmt"
	"io/ioutil"

	"github.com/ghodss/yaml"
)

type Auth struct {
	Audience string `json:"audience"`
}

type Endpoint struct {
	Source         string `json:"source"`
	KeyPath        string `json:"keyPath"`
	Authentication *Auth  `json:"authentication,omitempty"`
}

type Config struct {
	FluxRecvVersion int        `json:"fluxRecvVersion"`
	API             string     `json:"api"`
	Endpoints       []Endpoint `json:"endpoints"`
}

func ConfigFromBytes(configBytes []byte) (Config, error) {
	var config Config

	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return config, err
	}
	if config.FluxRecvVersion != 1 {
		return config, fmt.Errorf("not a valid config file (field fluxRecvVersion != 1)")
	}

	return config, nil
}

func ConfigFromFile(path string) (Config, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return ConfigFromBytes(configBytes)
}
