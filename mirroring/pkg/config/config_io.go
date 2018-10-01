package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Reads config from default config location($HOME/.mera/config.json or from user-defined location
// Returns parsed config or error
func ReadDefaultConfig() (config *Config, err error) {

	config, err = parseConfig()

	if err != nil {
		println(err.Error())
	}

	if config.Server1 == nil || config.Server1.IsEmpty() ||
		config.Server2 == nil || config.Server2.IsEmpty() {

		return nil, errors.New("Credentials are not set. Please run config setup")
	}

	// bytes, _ := json.MarshalIndent(config, "", "\t")
	// println(string(bytes))

	return config, nil
}

// Gather config values and applies default values
func parseConfig() (config *Config, err error) {
	if viper.IsSet("configPath") {
		viper.SetConfigFile(viper.GetString("configPath"))
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("$HOME/.mirroring/")
		viper.SetConfigType("json")
	}

	setDefaults()

	viper.AutomaticEnv()
	viper.ReadInConfig()

	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func setDefaults() {
	// Root defaults
	viper.SetDefault("DefaultOptions.DefaultSource", "server1")
	viper.SetDefault("ThrowImmediately", true)

	// ListOptions defaults
	viper.SetDefault("ListOptions.DefaultOptions.DefaultSource", "server1")
	viper.SetDefault("ListOptions.DefaultOptions.ThrowImmediately", false)
	viper.SetDefault("ListOptions.Merge", false)

	// PutOptions defaults
	viper.SetDefault("PutOptions.DefaultOptions.DefaultSource", "server1")
	viper.SetDefault("PutOptions.DefaultOptions.ThrowImmediately", false)
	viper.SetDefault("PutOptions.CreateBucketIfNotExist", true)

	// GetObjectOptions defaults
	viper.SetDefault("GetObjectOptions.DefaultOptions.DefaultSource", "server2")
	viper.SetDefault("GetObjectOptions.DefaultOptions.ThrowImmediately", false)

	// CopyOptions defaults
	viper.SetDefault("CopyOptions.DefaultOptions.DefaultSource", "server1")
	viper.SetDefault("CopyOptions.DefaultOptions.ThrowImmediately", true)

	// DeleteOptions defaults
	viper.SetDefault("DeleteOptions.DefaultOptions.DefaultSource", "server1")
	viper.SetDefault("DeleteOptions.DefaultOptions.ThrowImmediately", true)
}
