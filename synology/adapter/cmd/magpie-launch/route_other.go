//go:build !linux

package main

// defaultRoutes is empty away from Linux: the launcher then falls back to the
// first real private address, which is all a build machine's tests need.
func defaultRoutes() []route { return nil }
