package main

// This module provides the structures and methods for the interaction with the IPFS daemon
// including the methods for getting the swarm, bucket, etc.
//
// Author: Marco Costa

import (
	"bytes"
	"context"
	"log"
	"math"
	"net"
	"os/exec"
	"strings"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/tidwall/gjson"
)

// The Kademlia bucket size
const bucketSize int = 20

var timeout time.Duration = 500 * time.Millisecond

type ipInfo struct {
	nation   string
	protocol string
}

type swarm struct {
	lastSwarm              []shell.SwarmConnInfo // Last swarm array
	lastSwarmLocationCount map[string]int        // Nation -> #Peer
	lastSwarmProcolCount   map[string]int        // Protocol -> #Peer
	min                    int64
	avg                    int64
	max                    int64
}

type bucket struct {
	lastBucket                [bucketSize]string
	lastBucketNewPeers        int
	lastBucketNewPeersOffline int
	lastBucketOffline         int
}

// Ipfs is the main structure for the IPFS node
type Ipfs struct {
	selfID string            // The ID of the local peer
	sh     *(shell.Shell)    // The shell for daemon interaction
	ipInfo map[string]ipInfo // IP -> ipInfo
	sw     swarm
	bu     bucket
}

// Given an IPFS address returns the IP address and the transport protocol used
func trimIpfsAddress(addr string) (string, string) {
	/* ["", ip version protocol, ip address, transport protocol, port] */
	split := strings.Split(addr, "/")
	return split[2], split[1]
}

// getIPCountry returns the ip country relative to "ip"
// Argument: the ip to check
func getIPCountry(ip string) string {
	res, err := client.GetCountry(net.ParseIP(ip))
	if err != nil {
		return "Undefined"
	}

	return strings.TrimRight(res, "\n")
}

// InitIPFS initializes a new Ipfs struct
// Arguments: address of the local API
func (i *Ipfs) InitIPFS() {
	i.sh = shell.NewShell(ipfsAPI)
	cmd := exec.Command("ipfs", "id")
	stdout, err := cmd.Output()
	if err != nil {
		log.Fatalf("The daemon isn't started on %v\nPlease start the daemon or modify \"ipfsAPI\" in config.go\n", ipfsAPI)
	}
	i.selfID = gjson.Get(string(stdout), "ID").String()
	i.ipInfo = make(map[string]ipInfo)
}

// GetSwarmInfos construct the infos (location, ipversion, rtt) for the last swarm
// Argument: n the iteration number
func (i *Ipfs) GetSwarmInfos(n int) {
	// Resetting memory
	// this is the faster way proposed by Golang docs
	i.sw.lastSwarmLocationCount = nil
	i.sw.lastSwarmLocationCount = make(map[string]int)
	i.sw.lastSwarmProcolCount = nil
	i.sw.lastSwarmProcolCount = make(map[string]int)
	i.sw.avg = int64(0)
	i.sw.min = int64(math.MaxInt64)
	i.sw.max = int64(0)
	found := 0

	/* saving backup */
	db.Swarm[n-1] = make([]string, len(i.sw.lastSwarm))

	for j := 0; j < len(i.sw.lastSwarm); j++ {
		ipAddr, ipProto := trimIpfsAddress(i.sw.lastSwarm[j].Addr)
		curr, ok := i.ipInfo[ipAddr]
		if ok {
			// Location known previously
		} else { // Location unknown
			curr.nation = getIPCountry(ipAddr)
			curr.protocol = ipProto
			i.ipInfo[ipAddr] = curr
		}
		// Add the location to the total count
		i.sw.lastSwarmLocationCount[curr.nation]++
		i.sw.lastSwarmProcolCount[curr.protocol]++

		// If enabled ping the address to add his RTT to the stats
		if !enablePings {
			continue
		}
		rtt, err := getRTT(ipAddr)
		if err != nil || rtt == 0 {
			log.Printf("[!!] can't get %v RTT", ipAddr)
			continue
		}
		// Compute the MIN, MAX, AVG stats
		rttNano := rtt.Nanoseconds()
		i.sw.avg += rttNano
		found++
		if rttNano < i.sw.min {
			i.sw.min = rttNano
		}
		if rttNano > i.sw.max {
			i.sw.max = rttNano
		}
	}

	if found > 0 {
		i.sw.avg = i.sw.avg / int64(found)
	}
}

// GetSwarmPeers gets the current swarm peers and saves it in the lastSwarm array
func (i *Ipfs) GetSwarmPeers() {
	info, err := i.sh.SwarmPeers(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	i.sw.lastSwarm = info.Peers
}

// GetBucket inserts in i.lastBucket the Kademlia bucket relative to peerID
// 			 using the command ipfs dht query
// Argument the peerID, the dimension of the maximum returned array
// Returns an error if the command wasn't executed, nil otherwise
func (i *Ipfs) GetBucket(peerID string, dim int) ([]string, error) {
	// Initializing the exec structures for the ipfs dht query command
	cmd := exec.Command("ipfs", "dht", "query", peerID)
	// Redirecting the output on the out Buffer
	var out bytes.Buffer
	cmd.Stdout = &out
	// Starting the command and waiting for the process termination by timeout or normal ending
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil {
			log.Fatal("failed to kill process: ", err)
		}
	case err := <-done:
		if err != nil {
			return nil, err
		}
	}

	// Dividing the output in a string array, one for every PeerID
	t := strings.Split(out.String(), "\n")

	var newBucket []string
	if len(t) < bucketSize {
		newBucket = t[:]
	} else {
		newBucket = t[:bucketSize]
	}

	// Statistic computation of the new bucket found
	// Checking
	// 		1. number of new peers in the bucket
	// 		2. number of old peers no more reachables
	i.bu.lastBucketNewPeers = 0
	i.bu.lastBucketNewPeersOffline = 0
	i.bu.lastBucketOffline = 0
OuterLoop:
	for _, v := range i.bu.lastBucket {
		if v == "" {
			continue
		}
		for _, k := range newBucket {
			if v == k {
				// This peer was in the old bucket
				i.bu.lastBucketNewPeers++
				/* continue OuterLoop */
				break
			}
		}
		// Peer v isn't in the new bucket, still reachable?
		// if the node isn't reachable function ID does not have a public ip address related
		a, err := i.sh.ID(v)
		i.bu.lastBucketNewPeersOffline++
		if err != nil || a == nil {
			continue
		}
		for _, ipfsAddr := range a.Addresses {
			if ipfsAddr == "" {
				continue
			}
			ipAddr, _ := trimIpfsAddress(ipfsAddr)
			netIPAddr := net.ParseIP(ipAddr)
			if netIPAddr != nil && !isPrivateIP(netIPAddr) {
				i.bu.lastBucketNewPeersOffline--
				continue OuterLoop
			}
		}
		i.bu.lastBucketOffline++
	}

	i.bu.lastBucketNewPeers = 20 - i.bu.lastBucketNewPeers

	// Copy the new bucket in the structure and return
	copy(i.bu.lastBucket[:], newBucket)
	return t, nil
}
