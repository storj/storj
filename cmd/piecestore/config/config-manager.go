package configManager

import (
	"errors"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// Manager holds the config filepath
type Manager struct {
	path string
}

// Config holds the contents of the config file
type Config struct {
	NodeID string
	IP     string
	Port   string
}

// New creates new Manager
func New(path string) *Manager {
	return &Manager{path: path}
}

// NewConfig creates new Config
func NewConfig(c Config) *Config {
	return &Config{IP: c.IP, Port: c.Port, NodeID: c.NodeID}
}

// WriteConfig marshals config data to YAML and writes it to config file
func (m *Manager) WriteConfig(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	file, err := os.Create(m.path)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// ReadConfig unmarshals and returns config YAML data
func (m *Manager) ReadConfig() (*Config, error) {
	var config *Config

	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		return nil, errors.New("Config file does not exist")
	}

	data, err := ioutil.ReadFile(m.path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config, err
}
