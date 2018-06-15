package main

import (
	"os"

	yaml "gopkg.in/yaml.v2"
)

// AppConfig ...
type AppConfig struct {
	HTTPAddress     string   `yaml:"httpAddress"`
	UpstreamServers []string `yaml:"upstreamServers"`
	ProxyStrategy   string   `yaml:"proxyStrategy"`
	IPHeaderName    string   `yaml:"ipHeaderName"`
	CORSOrigins     []string `yaml:"corsOrigins"`
}

// ReadYAMLConfig ...
func ReadYAMLConfig(path string, config interface{}) error {
	r, err := os.Open(path)
	if err != nil {
		return err
	}
	defer r.Close()

	return yaml.NewDecoder(r).Decode(config)
}
