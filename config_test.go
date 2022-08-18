package main

import (
	"bytes"
	"net"
	"reflect"
	"testing"
)

func TestReadConfigFromReader(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		want    *Config
		wantErr bool
	}{
		{
			name: "empty binds",
			config: `
hosts:
  - name: test
    mac: 00-01-02-03-04-05
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid bind (no port)",
			config: `
binds:
  - 0.0.0.0
hosts:
  - name: test
    mac: 00-01-02-03-04-05
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid bind (invalid ip)",
			config: `
binds:
  - example.com:9
hosts:
  - name: test
    mac: 00-01-02-03-04-05
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid bind (invalid port - out of range)",
			config: `
binds:
  - 127.0.0.1:0
hosts:
  - name: test
    mac: 00-01-02-03-04-05
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid bind (invalid port - not a number)",
			config: `
binds:
  - 127.0.0.1:http
hosts:
  - name: test
    mac: 00-01-02-03-04-05
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty hosts",
			config: `
binds:
  - 127.0.0.1:9
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid host (invalid mac)",
			config: `
binds:
  - 127.0.0.1:9
hosts:
  - name: test
    mac: ABCDEFGH
    run: /bin/false
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid host (empty run)",
			config: `
binds:
  - 127.0.0.1:9
hosts:
  - name: test
    mac: 00-01-02-03-04-05
`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "everything correct",
			config: `
binds:
  - 127.0.0.1:9
  - :7
hosts:
  - name: joe
    mac: 00-01-02-03-04-05
    run: /bin/false
  - name: alice
    mac: 01-01-02-03-04-05
    run: /bin/false
  - name: wildcard
    run: /bin/false
`,
			want: &Config{
				Binds: []string{
					"127.0.0.1:9",
					":7",
				},
				Hosts: []Host{
					{
						Name:      "joe",
						Mac:       "00-01-02-03-04-05",
						Run:       []string{"/bin/false"},
						parsedMac: []byte{0, 1, 2, 3, 4, 5},
					},
					{
						Name:      "alice",
						Mac:       "01-01-02-03-04-05",
						Run:       []string{"/bin/false"},
						parsedMac: []byte{1, 1, 2, 3, 4, 5},
					},
					{
						Name:      "wildcard",
						Mac:       "",
						Run:       []string{"/bin/false"},
						parsedMac: nil,
					},
				},
				parsedBinds: []net.UDPAddr{
					{IP: net.ParseIP("127.0.0.1"), Port: 9, Zone: ""},
					{IP: nil, Port: 7, Zone: ""},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadConfigFromReader(bytes.NewReader([]byte(tt.config)))
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadConfigFromReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadConfigFromReader() got = %v, want %v", got, tt.want)
			}
		})
	}
}
