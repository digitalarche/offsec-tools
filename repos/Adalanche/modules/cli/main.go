package cli

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/felixge/fgtrace"
	"github.com/lkarlslund/adalanche/modules/ui"
	"github.com/lkarlslund/adalanche/modules/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	Root = &cobra.Command{
		Use:              "adalanche",
		Short:            version.VersionStringShort(),
		SilenceErrors:    true,
		SilenceUsage:     true,
		TraverseChildren: true,
	}
	prerunhooks []func(cmd *cobra.Command, args []string) error

	loglevel = Root.Flags().String("loglevel", "info", "Console log level")

	logfile      = Root.Flags().String("logfile", "", "File to log to")
	logfilelevel = Root.Flags().String("logfilelevel", "info", "Log file log level")
	logzerotime  = Root.Flags().Bool("logzerotime", false, "Logged timestamps start from zero when program launches")

	embeddedprofiler  = Root.Flags().Bool("embeddedprofiler", false, "Start embedded Go profiler on localhost:6060")
	cpuprofile        = Root.Flags().Bool("cpuprofile", false, "Save CPU profile from start to end of processing in datapath")
	dofgtrace         = Root.Flags().Bool("fgtrace", false, "Save CPU trace start to end of processing in datapath")
	cpuprofiletimeout = Root.Flags().Int32("cpuprofiletimeout", 0, "CPU profiling timeout in seconds (0 means no timeout)")

	// also available for subcommands
	Datapath = Root.Flags().String("datapath", "data", "folder to store and read data")

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show adalanche version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Info().Msg(version.ProgramVersionShort())
			return nil
		},
	}

	OverrideArgs   []string
	stopprofile    = make(chan bool, 5)
	stopfgtrace    = make(chan bool, 5)
	profilewriters sync.WaitGroup
)

func bindFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && viper.IsSet(f.Name) {
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				sv.Replace(viper.GetStringSlice(f.Name))
			} else {
				f.Value.Set(viper.GetString(f.Name))
			}
		}
	})
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && viper.IsSet(f.Name) {
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				sv.Replace(viper.GetStringSlice(f.Name))
			} else {
				f.Value.Set(viper.GetString(f.Name))
			}
		}
	})
	for _, subCommand := range cmd.Commands() {
		bindFlags(subCommand)
	}
}

func loadConfiguration(cmd *cobra.Command) {
	// Bind environment variables
	viper.SetEnvPrefix("ADALANCHE_")
	viper.AutomaticEnv()

	// Use config file from the flag.
	viper.SetConfigFile(filepath.Join(*Datapath, "configuration.yaml"))
	if err := viper.ReadInConfig(); err == nil {
		ui.Info().Msgf("Using configuration file: %v", viper.ConfigFileUsed())
	} else {
		ui.Debug().Msgf("No settings loaded: %v", err.Error())
	}

	bindFlags(cmd)
}

func init() {
	cobra.OnInitialize(func() {
		loadConfiguration(Root)
	})

	Root.AddCommand(versionCmd)
	Root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		ui.Zerotime = *logzerotime

		ll, err := ui.LogLevelString(*loglevel)
		if err != nil {
			ui.Error().Msgf("Invalid log level: %v - use one of: %v", *loglevel, ui.LogLevelStrings())
		} else {
			ui.SetLoglevel(ll)
		}

		if *logfile != "" {
			ll, err = ui.LogLevelString(*logfilelevel)
			if err != nil {
				ui.Error().Msgf("Invalid log file log level: %v - use one of: %v", *logfilelevel, ui.LogLevelStrings())
			} else {
				ui.SetLogFile(*logfile, ll)
			}
		}

		ui.Info().Msg(version.VersionString())

		if *embeddedprofiler {
			go func() {
				port := 6060
				for {
					err := http.ListenAndServe(fmt.Sprintf("localhost:%v", port), nil)
					if err != nil {
						ui.Error().Msgf("Profiling listener failed: %v, trying with new port", err)
						port++
					} else {
						break
					}
				}
				ui.Info().Msgf("Profiling listener started on port %v", port)
			}()
		}

		if *dofgtrace {
			tracefile := filepath.Join(*Datapath, "adalanche-fgtrace-"+time.Now().Format("06010215040506")+".json")
			trace := fgtrace.Config{Dst: fgtrace.File(tracefile)}.Trace()

			profilewriters.Add(1)

			go func() {
				<-stopfgtrace
				err = trace.Stop()
				if err != nil {
					ui.Error().Msgf("Problem stopping fgtrace: %v", err)
				}
				profilewriters.Done()
			}()

			if *cpuprofiletimeout > 0 {
				go func() {
					<-time.After(time.Second * (time.Duration(*cpuprofiletimeout)))
					stopfgtrace <- true
				}()
			}

		}

		if *cpuprofile {
			pproffile := filepath.Join(*Datapath, "adalanche-cpuprofile-"+time.Now().Format("06010215040506")+".pprof")
			f, err := os.Create(pproffile)
			if err != nil {
				return fmt.Errorf("Could not set up CPU profiling in file %v: %v", pproffile, err)
			}
			pprof.StartCPUProfile(f)

			profilewriters.Add(1)

			go func() {
				<-stopprofile
				pprof.StopCPUProfile()
				profilewriters.Done()
			}()

			if *cpuprofiletimeout > 0 {
				go func() {
					<-time.After(time.Second * (time.Duration(*cpuprofiletimeout)))
					stopprofile <- true
				}()
			}
		}

		debug.SetGCPercent(10)

		// Ensure the data folder is available
		if _, err := os.Stat(*Datapath); os.IsNotExist(err) {
			err = os.MkdirAll(*Datapath, 0711)
			if err != nil {
				return fmt.Errorf("Could not create data folder %v: %v", Datapath, err)
			}
		}
		for _, prerunhook := range prerunhooks {
			err := prerunhook(cmd, args)
			if err != nil {
				return fmt.Errorf("Prerun hook failed: %v", err)
			}
		}

		return nil
	}
	Root.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		stopfgtrace <- true
		stopprofile <- true
		profilewriters.Wait()
		return nil
	}
}

func AddPreRunHook(f func(cmd *cobra.Command, args []string) error) {
	prerunhooks = append(prerunhooks, f)
}

func CliMainEntryPoint() error {
	if len(os.Args[1:]) == 0 {
		Root.SetArgs(OverrideArgs)
	}

	err := Root.Execute()

	if err == nil {
		ui.Info().Msgf("Terminating successfully")
	}

	return err
}