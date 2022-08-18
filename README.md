# cwolsrv
[![Actions Status](https://github.com/Eun/cwolsrv/workflows/push/badge.svg)](https://github.com/Eun/cwolsrv/actions)
[![Coverage Status](https://coveralls.io/repos/github/Eun/cwolsrv/badge.svg?branch=master)](https://coveralls.io/github/Eun/cwolsrv?branch=master)
[![PkgGoDev](https://img.shields.io/badge/pkg.go.dev-reference-blue)](https://pkg.go.dev/github.com/Eun/cwolsrv)
[![go-report](https://goreportcard.com/badge/github.com/Eun/cwolsrv)](https://goreportcard.com/report/github.com/Eun/cwolsrv)
---
Run custom commands on wake on lan magic packets.  
This application listens for wake on lan magic packets and
runs a specified command when it sees a known host.


## Installation
1. Go to the latest releases [here](https://github.com/Eun/cwolsrv/releases/), and download the correct binary for your system
2. Create a config file in `/etc/cwolsrv/cwolsrv.yaml`
   ```yaml
    # specify on which interfaces to listen on (UDP only)
    binds:
      - :9     # listen on any interface on port 9
      - :7     # listen on any interface on port 7
    
    # specify which hosts are known and what to do
    # when a matching magic packet arrives
    hosts:
      # run /bin/true when a magic packet is sent to mac 01-02-03-04-05-06 
      - name: Joe's Computer   # choose name for the host (optional)
        mac: 01:02:03:04:05:06 # the mac address to look out for 
        run:                   # run this command
          - /bin/true
      # run /bin/true for every magic packet that is arriving
      - name: test
        run:
          - /bin/true
    ```
3. Run `cwolsrv`


## Scripting
> Notice that every application that has been invoked by `cwolsrv` will get killed after 10 seconds.
So make sure your script / application is running fast, or creates another backround process.

Every application that has been invoked by `cwolsrv` gets following
environment variables:

| Name        | Description                                 | Example             |
|-------------|---------------------------------------------|---------------------|
| `HOST_NAME` | the `name` of the host                      | `Joe's Computer`    |
| `HOST_MAC`  | the `mac` of the host                       | `01:02:03:04:05:06` |
| `MAC`       | the `mac` that was sent in the magic packet | `01:02:03:04:05:06` |


## Example Usecase
Mac Mini's wake from WoL packets, but they don't enable the display.
With a simple script (that will get invoked by `cwolsrv`) we can wake the display.
```shell
#!/bin/bash
ssh user@mac-mini.lan "caffeinate -u -t 1"
```