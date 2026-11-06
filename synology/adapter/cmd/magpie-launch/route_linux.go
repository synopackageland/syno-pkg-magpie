//go:build linux

package main

// defaultRoutes is Linux's IPv4 routing table: the interface on the default
// route is the network this NAS is known by.
func defaultRoutes() []route { return readProcRoutes("/proc/net/route") }
