// Command magpie-launch starts magpie with the address other machines reach
// this NAS at, so magpie's own panel does not have to guess.
//
// `magpie serve` advertises every non-loopback IPv4 it finds. On a NAS that
// runs containers, that list is mostly Docker bridges (172.17.0.0/16 and the
// like), and magpie's only input for it is the documented MAGPIE_PUBLIC_URL.
// This launcher works that value out per NAS, right before magpie starts:
// the interface that carries the default route, else the first real private
// address, never loopback, link-local, a container bridge or a proxy's fake-IP
// range. A MAGPIE_PUBLIC_URL set by the operator wins.
//
// It is a fixed argv program: it changes no argument, reads no request and
// execs magpie in place, so signals and the process the daemon supervises stay
// the same.
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// publicURLEnv is magpie's documented address for other machines.
const publicURLEnv = "MAGPIE_PUBLIC_URL"

// defaultPort is magpie's gateway port when MAGPIE_ADDR does not say one.
const defaultPort = 3425

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "magpie-launch: %v\n", err)
		os.Exit(1)
	}
	binary := filepath.Join(filepath.Dir(exe), "magpie")
	if fi, err := os.Stat(binary); err != nil || fi.IsDir() {
		fmt.Fprintf(os.Stderr, "magpie-launch: %s is missing beside this launcher\n", binary)
		os.Exit(1)
	}
	if os.Getenv(publicURLEnv) == "" {
		if url := publicURL(os.Getenv("MAGPIE_ADDR")); url != "" {
			os.Setenv(publicURLEnv, url)
		}
	}
	argv := append([]string{binary}, os.Args[1:]...)
	if err := syscall.Exec(binary, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "magpie-launch: %v\n", err)
		os.Exit(1)
	}
}

// publicURL is the address other machines reach this NAS at, or "" when there
// is none to advertise (a loopback-only gateway, or no usable address).
func publicURL(addr string) string {
	port := gatewayPort(addr)
	if port == 0 {
		return ""
	}
	host, ok := listenHost(addr)
	if ok && !usable(host) {
		// MAGPIE_ADDR is pinned to loopback, a bridge or the wildcard: only a
		// real address is worth advertising.
		if !isWildcard(host) {
			return ""
		}
	}
	ip := choosePrimary(defaultRoutes(), interfaces())
	if ip == "" {
		return ""
	}
	return "http://" + net.JoinHostPort(ip, strconv.Itoa(port))
}

// gatewayPort is the port MAGPIE_ADDR names, else magpie's default.
func gatewayPort(addr string) int {
	if addr == "" {
		return defaultPort
	}
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return 0
	}
	return n
}

// listenHost is MAGPIE_ADDR's host, and whether it names one.
func listenHost(addr string) (string, bool) {
	if addr == "" {
		return "", false
	}
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false
	}
	return h, true
}

func isWildcard(host string) bool {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

// usable: ip is worth advertising to another machine.
func usable(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil || ip.To4() == nil {
		return false
	}
	return !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !fakeIP(ip)
}

// fakeIP: a proxy's TUN or fake-ip address (198.18.0.0/15), which no other
// machine on the LAN can reach.
func fakeIP(ip net.IP) bool {
	_, block, _ := net.ParseCIDR("198.18.0.0/15")
	return block != nil && block.Contains(ip)
}

// virtual reports an interface that is not the NAS's own network: container
// bridges and veth pairs, VM bridges, tunnels and the loopback.
func virtual(name string) bool {
	for _, prefix := range []string{"lo", "docker", "br-", "veth", "virbr", "cni", "podman", "tun", "tap", "wg", "tailscale",
		"utun", "ipsec", "ppp", "awdl", "llw", "bridge", "gif", "stf", "anpi"} {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// ifaceAddr is one interface and the IPv4 addresses configured on it.
type ifaceAddr struct {
	Name  string
	Addrs []string
}

// route is one /proc/net/route row: the destination and the interface.
type route struct {
	Interface   string
	Destination string // hex, little-endian, as the kernel writes it
	Gateway     string
	Metric      string
}

// choosePrimary picks the address to advertise: the default route's interface
// first (that is the network the NAS is known by), then the first real private
// address on a non-virtual interface.
func choosePrimary(routes []route, ifs []ifaceAddr) string {
	byName := map[string][]string{}
	for _, i := range ifs {
		byName[i.Name] = i.Addrs
	}
	if name := defaultRouteInterface(routes); name != "" {
		if ip := firstUsable(byName[name]); ip != "" {
			return ip
		}
	}
	for _, i := range ifs {
		if virtual(i.Name) {
			continue
		}
		if ip := firstUsable(i.Addrs); ip != "" {
			return ip
		}
	}
	return ""
}

// defaultRouteInterface is the interface of the lowest-metric default route.
func defaultRouteInterface(routes []route) string {
	best, bestMetric := "", -1
	for _, r := range routes {
		if r.Destination != "00000000" || r.Gateway == "00000000" {
			continue
		}
		metric := metricOf(r)
		if best == "" || metric < bestMetric {
			best, bestMetric = r.Interface, metric
		}
	}
	return best
}

// firstUsable is the first address in addrs worth advertising.
func firstUsable(addrs []string) string {
	for _, a := range addrs {
		if usable(a) {
			return a
		}
	}
	return ""
}
