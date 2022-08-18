package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli"
)

var cliFlagLogFile = cli.StringFlag{
	Name:   "logfile",
	Usage:  "Logfile to write to",
	EnvVar: "LOGFILE",
	Value:  "",
}
var cliFlagDebug = cli.BoolFlag{
	Name:   "debug",
	Usage:  "Enable debug log",
	EnvVar: "DEBUG",
}

var cliFlagConfig = cli.StringFlag{
	Name:   "config",
	Usage:  "Config file to use",
	EnvVar: "CONFIG",
	Value:  "/etc/cwolsrv/cwolsrv.yaml",
}

const maxGracefulCloseTimeout = time.Second * 15

var (
	version string
	commit  string
	date    string
)

func main() {
	var logfile *os.File
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	app := cli.NewApp()
	app.Name = "cwolsrv"
	app.Version = version + " " + commit + " " + date
	app.Description = "Run custom commands on wake on lan magic packets."
	app.Author = "Tobias Salzmann"
	app.Flags = []cli.Flag{
		cliFlagLogFile,
		cliFlagDebug,
		cliFlagConfig,
	}
	app.Before = func(context *cli.Context) error {
		loglevel := zerolog.InfoLevel
		if context.Bool(cliFlagDebug.Name) {
			loglevel = zerolog.DebugLevel
		}
		log.Logger = log.Level(loglevel)

		logfilePath := context.String(cliFlagLogFile.Name)
		if logfilePath != "" {
			var err error
			//nolint:gomnd // allow magic number 0666
			logfile, err = os.OpenFile(logfilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0666)
			if err != nil {
				return errors.Wrap(err, "unable to create logfile")
			}
			log.Logger = log.Output(logfile)
		}
		log.Debug().Str("logfile", logfilePath).Msg("debug log enabled")
		return nil
	}
	app.After = func(context *cli.Context) error {
		if logfile == nil {
			return nil
		}
		return logfile.Close()
	}

	app.Action = func(cliContext *cli.Context) error {
		log.Debug().Msg("starting")

		config, err := ReadConfigFromFile(cliContext.String(cliFlagConfig.Name))
		if err != nil {
			return errors.Wrap(err, "unable to read config")
		}

		servers := make([]*Server, len(config.parsedBinds))

		log.Debug().Msg("creating servers")
		for i, bind := range config.parsedBinds {
			var err error
			servers[i], err = NewServer(bind, config)
			if err != nil {
				return errors.Wrapf(err, "unable to create new server for bind `%s'", bind.String())
			}
		}

		log.Debug().Msg("starting servers")

		for i := range servers {
			go func(i int) {
				if err := servers[i].Serve(); err != nil {
					log.Error().Err(err).Msgf("unable to start server with bind `%s'", servers[i].Bind.String())
				}
			}(i)
		}

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		<-c

		ctx, cancel := context.WithTimeout(context.Background(), maxGracefulCloseTimeout)
		defer cancel()
		log.Debug().Msg("shutting down")

		log.Debug().Msg("closing servers")
		var wg sync.WaitGroup
		wg.Add(len(servers))
		for i := range servers {
			go func(i int) {
				if err := servers[i].Close(ctx); err != nil {
					log.Error().Err(err).Msgf("unable to close server with bind `%s'", servers[i].Bind.String())
				}
				wg.Done()
			}(i)
		}

		wg.Wait()

		log.Debug().Msg("shutdown complete")
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error")
	}
	os.Exit(0)
}
