/*
Copyright © 2026 bensoer

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"stfg/internal/storage"
	"stfg/internal/storage/json"
)

// storageCtxKey is the context key used to pass the storage.Storage
// handle from the groceries command's PersistentPreRunE to subcommand Run handlers.
type storageCtxKey struct{}

// getStore retrieves the storage.Storage handle from the cobra command context.
// The storage handle is set by the root command's PersistentPreRunE, so every
// subcommand has access to it. If the handle is missing (should not happen in
// normal operation) a fresh store is created as a fallback.
func getStore(cmd *cobra.Command) storage.Storage {
	store, ok := cmd.Context().Value(storageCtxKey{}).(storage.Storage)
	if ok && store != nil {
		return store
	}
	// Fallback: initialize a fresh store. This should only trigger if
	// PersistentPreRunE failed silently or during testing.
	store, err := json.NewJSON(cmd.Context())
	if err != nil {
		// Last resort — returning nil will cause a panic at the call site
		// which is better than silently proceeding without storage.
		return nil
	}
	ctx := context.WithValue(cmd.Context(), storageCtxKey{}, store)
	cmd.SetContext(ctx)
	return store
}

var cfgFile string
var logFile string
var quiet bool
var logLevel string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stfg",
	Short: "A CLI tool for searching the best deals on groceries from flyers and stores",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.stfg.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		logger, err := setUpLogger()
		if err != nil {
			return err
		}

		zap.ReplaceGlobals(logger.Desugar())

		// Initialize storage for every command. The handle is stored on
		// the command context so subcommands can retrieve it via getStore().
		store, err := json.NewJSON(cmd.Context())
		if err != nil {
			return err
		}
		ctx := context.WithValue(cmd.Context(), storageCtxKey{}, store)
		cmd.SetContext(ctx)

		return nil
	}

	rootCmd.PersistentFlags().StringVarP(&logFile, "log-file", "", "", "The log file where to store the log output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Mute the log output from stdout/stderr")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", zap.InfoLevel.String(), "The log level")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".stfg" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".stfg")
	}

	viper.SetEnvPrefix("STFG")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func setUpLogger() (*zap.SugaredLogger, error) {

	// Setup the logger output
	if len(logFile) == 0 {
		//logFile = "karr-" + time.Now().Format("20060102T150405") + ".log"
		logFile = "stfg.log"
	} else {
		basePath := path.Dir(logFile)
		if err := os.MkdirAll(basePath, 0777); err != nil {
			return nil, err
		}
	}
	var f *os.File
	var err error

	if f, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
		// Using fmt to print to stdout since logger is not ready
		fmt.Println(err)
		return nil, err
	}

	// configuration settings
	pecfg := zap.NewProductionEncoderConfig()
	pecfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// Configure output encoders
	fileEncoder := zapcore.NewJSONEncoder(pecfg)
	consoleEncoder := zapcore.NewConsoleEncoder(pecfg)

	// Configure log levels
	atomicLevel, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, err
	}

	// Configure outputs based on params
	var core zapcore.Core
	if quiet {
		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(f), atomicLevel),
		)
	} else {
		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(f), atomicLevel),
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atomicLevel),
		)
	}

	//Build the logger
	logger := zap.New(core).Sugar()

	return logger, nil
}
