package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func sendMagicPaket(w io.Writer, mac net.HardwareAddr) error {
	mp := MagicPacket{
		Header:  [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		Payload: [16]MACAddress{},
	}

	var m MACAddress
	copy(m[:], mac[:6])
	for i := range mp.Payload {
		mp.Payload[i] = m
	}

	return binary.Write(w, binary.BigEndian, mp)
}

const testFilePrefix = ".cwolsrv-test-"

//nolint:gochecknoinits // allow inits, we need them to call ourselves
func init() {
	if name := os.Getenv("HOST_NAME"); name != "" {
		fmt.Println("WRITING " + testFilePrefix + name)
		f, err := os.Create(testFilePrefix + name)
		if err != nil {
			panic(errors.Wrap(err, "unable to create test file"))
		}

		if err := yaml.NewEncoder(f).Encode(map[string]string{
			"HOST_NAME": name,
			"HOST_MAC":  os.Getenv("HOST_MAC"),
			"MAC":       os.Getenv("MAC"),
		}); err != nil {
			panic(errors.Wrap(err, "unable to marshal result"))
		}

		if err := f.Close(); err != nil {
			panic(errors.Wrap(err, "unable to close test file"))
		}
		fmt.Println("DONE")
		os.Exit(0)
	}
}

func TestServer(t *testing.T) {
	test := func(t *testing.T, name, listenForMac string, sendMac net.HardwareAddr) {
		// delete test files
		os.Remove(testFilePrefix + name)

		exe, err := os.Executable()
		require.NoError(t, err)
		config, err := ReadConfigFromReader(bytes.NewReader([]byte(fmt.Sprintf(`
binds: [127.0.0.1:9000]
hosts:
  - name: %s
    mac: %s
    run: 
      - %s
`, name, listenForMac, exe))))
		require.NoError(t, err)

		server, err := NewServer(config.parsedBinds[0], config)
		require.NoError(t, err)

		go func() {
			require.NoError(t, server.Serve())
		}()

		// wait a bit before sending the message
		time.Sleep(time.Second)

		conn, err := net.DialUDP("udp", nil, &config.parsedBinds[0])
		require.NoError(t, err)

		require.NoError(t, sendMagicPaket(conn, sendMac))
		require.NoError(t, conn.Close())

		// wait a bit before checking if the test file exists
		time.Sleep(time.Second)

		f, err := os.Open(testFilePrefix + name)
		require.NoError(t, err)
		var data map[string]string
		require.NoError(t, yaml.NewDecoder(f).Decode(&data))
		require.NoError(t, f.Close())

		require.Equal(t, map[string]string{
			"HOST_NAME": name,
			"HOST_MAC":  listenForMac,
			"MAC":       sendMac.String(),
		}, data)

		require.NoError(t, server.Close(context.Background()))

		os.Remove(testFilePrefix + name)
	}

	t.Run("specific mac", func(t *testing.T) {
		test(t, "specific_mac", "00:01:02:03:04:05", net.HardwareAddr{0, 1, 2, 3, 4, 5})
	})

	t.Run("wildcard mac", func(t *testing.T) {
		test(t, "wildcard_mac", "", net.HardwareAddr{0, 1, 2, 3, 4, 5})
	})
}
