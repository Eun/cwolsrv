package main

import (
	"io"
	"net"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Binds []string `yaml:"binds"`
	Hosts []Host   `yaml:"hosts"`

	parsedBinds []net.UDPAddr
}

type Host struct {
	Name      string   `yaml:"name"`
	Mac       string   `yaml:"mac"`
	Run       []string `yaml:"run"`
	parsedMac net.HardwareAddr
}

func ReadConfigFromFile(path string) (*Config, error) {
	logger := log.With().Str("component", "config").Str("path", path).Logger()
	logger.Debug().Msg("opening config file")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug().Msg("file not found")
			return nil, errors.Errorf("file `%s' not found", path)
		}
		logger.Debug().Msg("error during opening")
		return nil, errors.Wrapf(err, "unable to open file `%s'", path)
	}
	defer f.Close()
	return ReadConfigFromReader(f)
}

func ReadConfigFromReader(r io.Reader) (*Config, error) {
	logger := log.With().Str("component", "config").Logger()

	var config Config
	logger.Debug().Msg("decoding config")
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "unable to decode config with yaml decoder")
	}
	logger.Debug().Msg("checking config for sanity")

	logger.Debug().Msg("checking binds")
	if len(config.Binds) == 0 {
		logger.Debug().Msg("no binds specified")
		return nil, errors.New("no binds specified")
	}

	config.parsedBinds = make([]net.UDPAddr, len(config.Binds))

	for i, s := range config.Binds {
		logger.Debug().Msgf("checking bind `%s'", s)
		host, port, err := net.SplitHostPort(s)
		if err != nil {
			logger.Debug().Msgf("bind `%s' is invalid (unable to split host port)", s)
			return nil, errors.Wrapf(err, "bind `%s' is invalid: use host:port as format", s)
		}

		ip := net.ParseIP(host)
		if host != "" && ip == nil {
			logger.Debug().Msgf("bind `%s' is invalid: `%s' is not a valid ip", s, host)
			return nil, errors.Errorf("bind `%s' is invalid: `%s' is not a valid ip", s, host)
		}

		nport, err := strconv.Atoi(port)
		if err != nil {
			logger.Debug().Err(err).Msgf("bind `%s' is invalid: `%s' is not a valid port", s, port)
			return nil, errors.Wrapf(err, "bind `%s' is invalid: `%s' is not a valid port", s, port)
		}

		if nport == 0 || nport >= 65536 {
			logger.Debug().Msgf("bind `%s' is invalid: `%s' is not a valid port", s, port)
			return nil, errors.Errorf("bind `%s' is invalid: `%s' is not a valid port", s, port)
		}

		config.parsedBinds[i] = net.UDPAddr{
			IP:   ip,
			Port: nport,
		}
	}
	logger.Debug().Msg("checking binds complete")

	logger.Debug().Msg("checking hosts")
	if len(config.Hosts) == 0 {
		logger.Debug().Msg("no hosts specified")
		return nil, errors.New("no hosts specified")
	}

	for i := range config.Hosts {
		var err error
		lgr := logger.With().
			Str("name", config.Hosts[i].Name).
			Str("mac", config.Hosts[i].Mac).
			Strs("run", config.Hosts[i].Run).Logger()
		lgr.Debug().Msg("checking host")
		if config.Hosts[i].Mac != "" {
			if config.Hosts[i].parsedMac, err = net.ParseMAC(config.Hosts[i].Mac); err != nil {
				lgr.Debug().Msgf("host is invalid: `%s' is not a valid mac address", config.Hosts[i].Mac)
				return nil, errors.Errorf("host `%s' is invalid: `%s' is not a valid mac address", config.Hosts[i].Name, config.Hosts[i].Mac)
			}
		}
		if len(config.Hosts[i].Run) == 0 {
			lgr.Debug().Msg("host is invalid: run cannot be empty")
			return nil, errors.Errorf("host `%s' is invalid: run cannot be empty", config.Hosts[i].Name)
		}
	}
	logger.Debug().Msg("checking hosts complete")

	return &config, nil
}
