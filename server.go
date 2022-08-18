package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Server struct {
	Bind             net.UDPAddr
	Config           *Config
	listener         *net.UDPConn
	stopCh           chan struct{}
	workingDirectory string
}

type MACAddress [6]byte

type MagicPacket struct {
	Header  [6]byte
	Payload [16]MACAddress
}

const maxRunTimeout = time.Second * 10

func NewServer(bind net.UDPAddr, config *Config) (*Server, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get working directory")
	}
	return &Server{
		Bind:             copyUDPAddr(bind),
		Config:           config,
		stopCh:           make(chan struct{}),
		workingDirectory: wd,
	}, nil
}

func (s *Server) Serve() error {
	var err error
	log.Debug().Str("addr", s.Bind.String()).Msg("listening")
	s.listener, err = net.ListenUDP("udp", &s.Bind)
	if err != nil {
		return errors.Wrapf(err, "unable to listen on `%s'", s.Bind.String())
	}

	readerChan := make(chan struct{})

	var buf [6 + 16*6]byte
	var size int
	var remote *net.UDPAddr
readSocket:
	for {
		go func() {
			size, remote, err = s.listener.ReadFromUDP(buf[:])
			readerChan <- struct{}{}
		}()

		select {
		case <-readerChan:
			if err != nil {
				log.Error().Err(err).Msg("unable to read incoming packet")
				continue readSocket
			}
			var mp MagicPacket
			log.Debug().Int("size", size).Str("remote", remote.String()).Msg("incoming packet")
			if err = binary.Read(bytes.NewReader(buf[:]), binary.BigEndian, &mp); err != nil {
				log.Error().Err(err).Msg("unable to read incoming packet")
				continue readSocket
			}
			mac := mp.Payload[0][:]
			// check sanity
			for i := 1; i < len(mp.Payload); i++ {
				if !bytes.Equal(mac, mp.Payload[i][:]) {
					log.Error().Err(err).Msg("malformed packet")
					continue readSocket
				}
			}

			addr := net.HardwareAddr(mac)
			log.Debug().Interface("mac", addr.String()).Msg("incoming magic packet")

			s.runCommands(addr)

		case <-s.stopCh:
			return nil
		}
	}
}

func (s *Server) Close(context.Context) error {
	s.stopCh <- struct{}{}
	return s.listener.Close()
}

func (s *Server) runCommands(mac net.HardwareAddr) {
	for i := range s.Config.Hosts {
		if s.Config.Hosts[i].parsedMac == nil || bytes.Equal(s.Config.Hosts[i].parsedMac, mac) {
			s.runCommand(&s.Config.Hosts[i], mac.String())
		}
	}
}

func (s *Server) runCommand(host *Host, mac string) {
	if len(host.Run) == 0 {
		// safety net
		return
	}
	lgr := log.With().
		Str("name", host.Name).
		Str("mac", host.Mac).
		Strs("run", host.Run).Logger()
	lgr.Debug().Msgf("running command for `%s'", mac)
	ctx, cancel := context.WithTimeout(context.Background(), maxRunTimeout)
	defer cancel()

	//nolint: gosec // allow commands pass through
	cmd := exec.CommandContext(ctx, host.Run[0])
	if len(host.Run) > 1 {
		cmd.Args = append(cmd.Args, host.Run[1:]...)
	}
	cmd.Stdout = lgr
	cmd.Stderr = lgr
	cmd.Dir = s.workingDirectory
	cmd.Env = append(cmd.Env, "MAC="+mac, "HOST_MAC="+host.parsedMac.String(), "HOST_NAME="+host.Name)
	lgr.Debug().Interface("cmd", cmd).Msg("starting command")
	if err := cmd.Run(); err != nil {
		lgr.Error().Err(err).Msg("error during command execution")
		return
	}
	lgr.Debug().Msgf("command executed for `%s'", mac)
}

func copyUDPAddr(addr net.UDPAddr) net.UDPAddr {
	dst := net.UDPAddr{
		IP:   make(net.IP, len(addr.IP)),
		Port: addr.Port,
		Zone: addr.Zone,
	}
	copy(dst.IP, addr.IP)
	return dst
}
