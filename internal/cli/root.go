package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/JulienTant/blogwatcher/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "blogwatcher",
		Short:         "BlogWatcher - Track blog articles and detect new posts.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig(cmd)
		},
	}
	rootCmd.Version = version.Version
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.PersistentFlags().String("db", "", "Path to the SQLite database file (default: ~/.blogwatcher/blogwatcher.db)")

	rootCmd.AddCommand(newAddCommand())
	rootCmd.AddCommand(newRemoveCommand())
	rootCmd.AddCommand(newBlogsCommand())
	rootCmd.AddCommand(newScanCommand())
	rootCmd.AddCommand(newArticlesCommand())
	rootCmd.AddCommand(newReadCommand())
	rootCmd.AddCommand(newReadAllCommand())
	rootCmd.AddCommand(newUnreadCommand())
	return rootCmd
}

func initConfig(cmd *cobra.Command) error {
	viper.SetEnvPrefix("BLOGWATCHER")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	return viper.BindPFlags(cmd.Flags())
}

func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		if !isPrinted(err) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
