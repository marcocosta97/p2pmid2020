package main

// This module provides the script routine for bruteforcing the 256 PeerIDs relative to the Kademlia
// routing table of the local Peer
// IMPORTANT: this module hasn's been fully tested due to lack of computational power
//
// Author: Marco Costa

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ipfs/go-cid"
)

const returnedBucket int = 500
const bucketTimeout time.Duration = 1 * time.Minute

// Computes the dXOR function of two [32]byte arrays as defined in the libp2p specs
func dXor(id1, id2 [32]byte) int {
	tot := 0

	for i := 0; i < len(id1); i++ {
		val := id1[i] ^ id2[i]
		t := 8
		for val != 0 {
			val = val >> 1
			t--
		}
		tot += t
		if t != 8 {
			return tot
		}
	}
	return tot
}

// Allowed characters for base58
var letters = []rune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

type dhtStruct struct {
	BucketIdentifiers [256]string // Found identifiers per bucket
	foundBucket       int         // Found buckets
	peerIDHash        [32]byte    // Hash of the local peer
	voidCycles        int         // Test since last match
}

// This function calculates the Hash and the dXOR of the generated ID if
// the ID is valid, if it is and it covers a new bucket it's added to
// the struct and all its bucket is tested recursively
func (d *dhtStruct) testString(s []byte, i *Ipfs) {
	d.voidCycles++
	//_, err := i.sh.ID(string(s))
	_, err := cid.Decode(string(s))
	if err != nil { // The ID is not valid
		return
	}
	sHash := sha256.Sum256(s)
	t := dXor(d.peerIDHash, sHash)
	if t >= 256 {
		return
	}
	// Checks if the relative bucket has already a PeerID, otherwise
	// if it's a valid PeerID it executes ipfs dht query recursively
	if d.BucketIdentifiers[t] == "" {
		buck, err := i.GetBucket(string(s), returnedBucket)
		if err == nil {
			d.voidCycles = 0
			d.BucketIdentifiers[t] = string(s)
			d.foundBucket++
			fmt.Printf("#%v/%v -> %v\n", t, (256 - d.foundBucket), string(s))

			for _, v := range buck {
				if v != "" {
					d.testString([]byte(v), i)
				}
			}
		}
	}
}

// Routine for finding the identifiers, stops when they all are found
func (d *dhtStruct) findIdentifiers(i *Ipfs) {
	d.foundBucket = 0
	selfIDBytes := []byte(i.selfID)
	d.peerIDHash = sha256.Sum256(selfIDBytes)

	// Start with the swarm peer know
	i.GetSwarmPeers()
	for _, v := range i.sw.lastSwarm {
		d.testString([]byte(v.Peer), i)
	}

	newID := make([]byte, len(selfIDBytes))
	copy(newID, selfIDBytes)
	rand.Seed(time.Now().UnixNano())
	for d.foundBucket != 256 {
		// Generates a random ID
		for j := 2; j < len(selfIDBytes); j++ {
			if rand.Intn(2) == 1 { // Toss a coin
				newID[j] = byte(letters[rand.Intn(len(letters))])
				d.testString(newID, i)
				if d.voidCycles > 1000 {
					copy(newID, selfIDBytes)
					d.voidCycles = 0
				}
			}
		}
	}
}

// Initializes the data and starts the procedure
func dht() {
	var i Ipfs
	i.InitIPFS()
	var d dhtStruct

	timeout = bucketTimeout
	d.findIdentifiers(&i)

	json, err := json.MarshalIndent(d.BucketIdentifiers, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	writeDataFile(i.selfID+".json", json)
	fmt.Printf("PeerIDs printed in %v.json, exiting\n", i.selfID)
}
