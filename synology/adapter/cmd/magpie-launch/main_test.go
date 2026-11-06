package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The NAS is known by the network on its default route, not by a container
// bridge that happens to be listed first.
func TestChoosePrimaryFollowsTheDefaultRoute(t *testing.T) {
	routes := []route{
		{Interface: "docker0", Destination: "000011AC", Gateway: "00000000", Metric: "0"},
		{Interface: "eth0", Destination: "00000000", Gateway: "0101A8C0", Metric: "100"},
		{Interface: "eth1", Destination: "00000000", Gateway: "0101A8C0", Metric: "200"},
	}
	ifs := []ifaceAddr{
		{Name: "lo", Addrs: []string{"127.0.0.1"}},
		{Name: "docker0", Addrs: []string{"172.17.0.1"}},
		{Name: "eth1", Addrs: []string{"192.0.2.9"}},
		{Name: "eth0", Addrs: []string{"198.51.100.20"}},
	}
	if got := choosePrimary(routes, ifs); got != "198.51.100.20" {
		t.Fatalf("choosePrimary = %q, want 198.51.100.20 (the lowest-metric default route)", got)
	}
}

// Without a routing table, the first real private address wins: loopback,
// link-local, a container bridge and a proxy's fake IP are all passed over.
func TestChoosePrimarySkipsLoopbackAndContainerBridges(t *testing.T) {
	ifs := []ifaceAddr{
		{Name: "lo", Addrs: []string{"127.0.0.1"}},
		{Name: "docker0", Addrs: []string{"172.17.0.1"}},
		{Name: "br-1a2b", Addrs: []string{"172.18.0.1"}},
		{Name: "vethabc", Addrs: []string{"169.254.10.2"}},
		{Name: "tun0", Addrs: []string{"198.18.0.1"}},
		{Name: "eth0", Addrs: []string{"fe80::1", "198.51.100.20"}},
	}
	if got := choosePrimary(nil, ifs); got != "198.51.100.20" {
		t.Fatalf("choosePrimary = %q, want 198.51.100.20", got)
	}
	if got := choosePrimary(nil, []ifaceAddr{{Name: "lo", Addrs: []string{"127.0.0.1"}}}); got != "" {
		t.Fatalf("choosePrimary on loopback alone = %q, want no address", got)
	}
}

// /proc/net/route's rows are read by position, and a row without a gateway is
// not a default route.
func TestReadProcRoutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route")
	table := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"docker0\t000011AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n"
	if err := os.WriteFile(path, []byte(table), 0o600); err != nil {
		t.Fatal(err)
	}
	routes := readProcRoutes(path)
	if len(routes) != 2 {
		t.Fatalf("read %d routes, want 2", len(routes))
	}
	if routes[0].Interface != "eth0" || routes[0].Destination != "00000000" || routes[0].Gateway != "0101A8C0" || routes[0].Metric != "100" {
		t.Fatalf("first route = %+v", routes[0])
	}
	if got := defaultRouteInterface(routes); got != "eth0" {
		t.Fatalf("defaultRouteInterface = %q, want eth0", got)
	}
	if got := readProcRoutes(filepath.Join(t.TempDir(), "absent")); got != nil {
		t.Fatalf("a missing table returned %v", got)
	}
}

// The advertised address keeps MAGPIE_ADDR's port and is dropped when the
// gateway is pinned to loopback: there is nothing another machine can reach.
func TestGatewayPortAndLoopbackPinning(t *testing.T) {
	cases := map[string]int{
		"":                  3425,
		"0.0.0.0:3425":      3425,
		"198.51.100.20:9000": 9000,
		"127.0.0.1:3425":    3425,
	}
	for addr, want := range cases {
		if got := gatewayPort(addr); got != want {
			t.Errorf("gatewayPort(%q) = %d, want %d", addr, got, want)
		}
	}
	for _, addr := range []string{"3425", "0.0.0.0", "0.0.0.0:0"} {
		if got := gatewayPort(addr); got != 0 {
			t.Errorf("gatewayPort(%q) = %d, want 0", addr, got)
		}
	}
	if isWildcard("198.51.100.20") {
		t.Fatal("a real address was taken for the wildcard")
	}
	if !isWildcard("") || !isWildcard("0.0.0.0") || !isWildcard("::") {
		t.Fatal("the wildcard forms were not recognised")
	}
	if usable("127.0.0.1") || usable("169.254.1.1") || usable("198.18.0.1") || usable("fe80::1") {
		t.Fatal("an unusable address passed")
	}
	if !usable("198.51.100.20") {
		t.Fatal("a private LAN address was refused")
	}
}
