package bustest

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

// hostPort is where a test reaches a container it just started.
//
// dockertest reports the published address from the daemon's point of view, and
// that is this machine only when the test and the daemon share one. In CI they
// do not: the job runs in a container with the host's /var/run/docker.sock bound
// in, so a container it starts is a SIBLING — published on the HOST's loopback,
// while the job's own 127.0.0.1 is a different interface with nothing on it. The
// server comes up, every dial is refused, and the retry loop blames the server:
//
//	bustest.go:51: dial tcp 127.0.0.1:32769: connect: connection refused
//
// The port dockertest chose is right, so keep it and answer the host ourselves.
func hostPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return net.JoinHostPort(dockerHost(), port)
}

// dockerHost is loopback outside a container, and the host as seen from inside
// one. DOCKER_TEST_HOST overrides both, for a daemon that is somewhere else.
func dockerHost() string {
	if h := os.Getenv("DOCKER_TEST_HOST"); h != "" {
		return h
	}

	// /.dockerenv exists only inside a container.
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return "127.0.0.1"
	}

	// host.docker.internal is the documented name, but only Docker Desktop
	// publishes it; on a plain Linux daemon it does not resolve.
	if addrs, err := net.LookupHost("host.docker.internal"); err == nil && len(addrs) > 0 {
		return addrs[0]
	}

	// The default route is the bridge gateway, which IS the host from in here.
	if gw := defaultGateway(); gw != "" {
		return gw
	}

	return "127.0.0.1"
}

// defaultGateway reads the container's default route out of /proc. The kernel
// writes the address little-endian in hex, so 0100007F is 127.0.0.1.
func defaultGateway() string {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only

	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 3 || fields[1] != "00000000" { // destination 0.0.0.0
			continue
		}

		v, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			return ""
		}

		return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
	}

	return ""
}
