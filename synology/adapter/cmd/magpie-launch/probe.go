package main

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

// defaultRoutes are the IPv4 default routes, or nil when the table cannot be
// read (see route_linux.go and route_other.go).
func readProcRoutes(path string) []route {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []route
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first { // the header line
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}
		out = append(out, route{Interface: fields[0], Destination: fields[1], Gateway: fields[2], Metric: fields[6]})
	}
	return out
}

// interfaces are this computer's interfaces and their IPv4 addresses, in the
// order the kernel lists them.
func interfaces() []ifaceAddr {
	list, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]ifaceAddr, 0, len(list))
	for _, iface := range list {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		entry := ifaceAddr{Name: iface.Name}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					entry.Addrs = append(entry.Addrs, ip4.String())
				}
				continue
			}
			if ip, _, err := net.ParseCIDR(a.String()); err == nil {
				if ip4 := ip.To4(); ip4 != nil {
					entry.Addrs = append(entry.Addrs, ip4.String())
				}
			}
		}
		out = append(out, entry)
	}
	return out
}

// metricOf is a route's metric, 0 when unreadable.
func metricOf(r route) int {
	if n, err := strconv.Atoi(r.Metric); err == nil {
		return n
	}
	return 0
}
