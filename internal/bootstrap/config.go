package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func SetupConfiguration(configFile string) {

	// If explicity given the config file, use it. Otherwise use default lookup techniques
	if configFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".stfg" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".stfg")
	}

	// Set defaults
	viper.SetDefault("fly_finder.whitelist", []string{
		"Superstore",
		"Thrifty Foods",
		"Quality Foods",
		"Buy-Low Foods",
		"Country Grocer",
		"No Frills",
		"Pharmasave",
		"Shoppers Drug Mart",
		"Walmart",
		"Nesters Market",
		"Rexall",
	})
	viper.SetDefault("providers.openrouter.api_key", "")
	viper.SetDefault("providers.openai.api_key", "")
	viper.SetDefault("providers.ollama.host", "http://localhost:11434")
	viper.SetDefault("storage.sqlite.cache_dir", "")
	viper.SetDefault("storage.sqlite.sqlite_file_name", "stfg.sqlite.db")
	viper.SetDefault("storage.bolt.cache_dir", "")
	viper.SetDefault("storage.bolt.bolt_file_name", "stfg.bolt.db")

	// ENV var overrides and bindings
	viper.SetEnvPrefix("STFG")
	// Tell viper how to remap configuration keys to environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	viper.BindEnv("providers.openrouter.api_key")
	viper.BindEnv("providers.openai.api_key")
	viper.BindEnv("providers.ollama.host")
	viper.BindEnv("storage.sqlite.cache_dir")

	viper.BindEnv("storage.sqlite.sqlite_file_name")
	viper.BindEnv("storage.bolt.cache_dir")
	viper.BindEnv("storage.bolt.bolt_file_name")

	// If a config file is found, read it in.
	// config file overrides env vars
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

}
