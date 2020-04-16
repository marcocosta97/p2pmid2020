package main

// This module implements the RTT check for the queried IPs
// See config.go for the default parameters like the number of pings
//
// Author Marco Costa

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sparrc/go-ping"
)

var privateIPBlocks []*net.IPNet
var isInit bool = false
var pingCache map[string]time.Duration

// Initializes the ping module used in a manner similar to a singleton
func initPing() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
	if cachePings {
		pingCache = make(map[string]time.Duration)
	}
}

// Checks if an IP address is a private network IP or not
// Returns true if it is, false otherwise
func isPrivateIP(ip net.IP) bool {
	if !isInit {
		initPing()
		isInit = true
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// getRTT returns the average RTT of the "ip" IP
// An error is returned if the ping didn's succeed
//
// NOTE: the script must be executed in privileged mode
// 		 otherwise duration 0 is returned
func getRTT(ip string) (time.Duration, error) {
	if !isInit {
		initPing()
		isInit = true
	}
	if cachePings {
		if rtt, present := pingCache[ip]; present {
			return rtt, nil
		}
	}

	p, err := ping.NewPinger(ip)
	if err != nil {
		return 0, err
	}
	p.SetPrivileged(true)
	if !p.Privileged() {
		log.Printf("Can't ping without privileged mode")
		return 0, nil
	}

	p.Count = pingIterations
	p.Timeout = pingTimeout
	p.Run()
	stat := p.Statistics()

	if cachePings {
		pingCache[ip] = stat.AvgRtt
	}

	return stat.AvgRtt, nil
}
