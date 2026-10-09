package bootstrap

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func BootstrapCobraRoot() *cobra.Command {

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   "stfg",
		Short: "A CLI tool for searching the best deals on groceries from flyers and stores",
		Long:  ``,
		// SilenceErrors and SilenceUsage keep Cobra from printing the same failure
		// that Execute logs once. Flag and argument errors still reach Execute.
		SilenceErrors: true,
		SilenceUsage:  true,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
	}

	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.stfg.yaml)")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	rootCmd.PersistentFlags().String("log-file", "", "The log file where to store the log output")
	viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))

	rootCmd.PersistentFlags().Bool("quiet", false, "Mute the log output from stdout/stderr")
	viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))

	rootCmd.PersistentFlags().String("log-level", zap.InfoLevel.String(), "The log level")
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {

		SetupConfiguration(viper.GetString("config"))

		logger, err := SetUpLogger(
			viper.GetString("log-file"),
			viper.GetBool("quiet"),
			viper.GetString("log-level"),
			cmd.ErrOrStderr(),
		)
		if err != nil {
			return err
		}

		zap.ReplaceGlobals(logger.Desugar())

		// // Build the container registry and register all loaders.
		// reg := RegisterProviders(cmd.Context())
		// // Assign the registry to the command context so every subcommand can
		// // retrieve it via getRegistry(cmd).
		// ctx := context.WithValue(cmd.Context(), registryCtxKey{}, reg)
		// cmd.SetContext(ctx)

		return nil
	}

	return rootCmd
}
