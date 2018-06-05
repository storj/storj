package config

import (
	"errors"
	"io/ioutil"
	"os"
	"path"

	"gopkg.in/yaml.v2"

	"storj.io/storj/pkg/piecestore"
)

// Manager holds the config filepath
type Manager struct {
	dir string
}

// Config holds the contents of the config file
type Config struct {
	NodeID string
	IP     string
	Port   string
}

var configFile string = "config.yaml"

// NewManager creates new Manager
func NewManager(dir string) (*Manager, error) {
	manager := &Manager{dir: dir}

	_, err := os.Stat(path.Join(dir, configFile))
	if os.IsExist(err) {
		if err != nil {
			return nil, err
		}
		return manager, nil
	}

	config := New(Config{Port: "7777", NodeID: pstore.DetermineID()})
	if err := manager.WriteConfig(config); err != nil {
		return nil, err
	}

	return manager, nil
}

// New creates new Config
func New(c Config) *Config {
	return &Config{IP: c.IP, Port: c.Port, NodeID: c.NodeID}
}

// WriteConfig marshals config data to YAML and writes it to config file
func (m *Manager) WriteConfig(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	file, err := os.Create(path.Join(m.dir, configFile))
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

	if _, err := os.Stat(path.Join(m.dir, configFile)); os.IsNotExist(err) {
		return nil, errors.New("Config file does not exist")
	}

	data, err := ioutil.ReadFile(path.Join(m.dir, configFile))
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config, err
}
