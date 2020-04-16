package main

// This module provides the script routine for the mandatory part and the bucket churn optional
// the results are encoded in a json file with timestamp as name.
// The script can be closed in a sae way using an interrupt signal
// IMPORTANT: the IPFS daemon must be executing on "localhost:5001" for changing this parameter
// 			  @see config.go
//
// NOTE: for executing the ping the script must be executed in privileged mode, otherwise RTT 0 is returned
//
// Author: Marco Costa

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ipinfo/go-ipinfo/ipinfo"
	"github.com/sparrc/go-ping"
)

var client *ipinfo.Client

// Defines a backup matrix used for next evaluations purposes
type backupDB struct {
	Swarm [][]string
}

type rtt struct {
	AvgRTT int64
	MaxRTT int64
	MinRTT int64
}

// The JSON output data format
type outputData struct {
	DayTime        string
	HourTime       string
	ConnectedPeers int
	ProtocolCount  *map[string]int
	LocationCount  *map[string]int
	Bucket         *[bucketSize]string
	BucketChurn    int
	BucketNewPeers int
	RTTInfos       rtt
}

var db backupDB

// Initializes the JSON output structs
func (d *outputData) initData(i *Ipfs) {
	t := time.Now()
	d.DayTime = t.Format(jsonDayFormat)
	d.HourTime = t.Format(jsonHourFormat)
	d.ConnectedPeers = len(i.sw.lastSwarm)
	d.LocationCount = &i.sw.lastSwarmLocationCount
	d.ProtocolCount = &i.sw.lastSwarmProcolCount
	d.Bucket = &i.bu.lastBucket
	d.BucketChurn = i.bu.lastBucketChurn
	d.BucketNewPeers = i.bu.lastBucketNewPeers
	if enablePings {
		d.RTTInfos.AvgRTT = i.sw.avg / rttFormat
		d.RTTInfos.MaxRTT = i.sw.max / rttFormat
		d.RTTInfos.MinRTT = i.sw.min / rttFormat
	}
}

// Writes data to a file in APPEND | CREATE mode
func writeDataFile(filename string, data []byte) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close() // ignore error; Write error takes precedence
		log.Panicln(err)
	}
	if err := f.Close(); err != nil {
		log.Panicln(err)
	}
}

// Routine execution
func routine(ipfs *Ipfs, i int) {
	ipfs.GetSwarmPeers()
	ipfs.GetSwarmInfos(i)
	ipfs.GetBucket(ipfs.selfID, bucketSize)

	var d outputData
	d.initData(ipfs)

	jsonString, _ := json.MarshalIndent(d, "", "	")

	jsonString = append(jsonString, byte(','))
	writeDataFile(dataFilename, jsonString)
	log.Printf("#%v/%v read successful - #swarm peers %v\n", i, maxExecutions, d.ConnectedPeers)
}

// This is the script method which handles the routine execution
func script() {
	var ipfs Ipfs
	ipfs.InitIPFS()
	authTransport := ipinfo.AuthTransport{Token: apiToken}
	httpClient := authTransport.Client()
	client = ipinfo.NewClient(httpClient)

	db.Swarm = make([][]string, maxExecutions)

	log.Println("PeerID: ", ipfs.selfID)

	writeDataFile(dataFilename, []byte("["))

	// Listening to signals
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGTERM)
	// First execution before using the ticker
	ticker := time.NewTicker(routineInterval)
	routine(&ipfs, 1)
	// Routine
R:
	for i := 2; i <= maxExecutions; i++ {
		select {
		case z := <-sig:
			log.Printf("received %v, graceful closing started", z)
			break R
		case <-ticker.C:
			routine(&ipfs, i)
		}
	}

	// End of routine
	ticker.Stop()
	writeDataFile(dataFilename, []byte("]"))

	// Saving the db for next evaluations
	dbJSON, err := json.MarshalIndent(db, "", "	")
	if err != nil {
		log.Panic(err)
	}
	writeDataFile("db.json", dbJSON)

	log.Println("script successfully terminated")
	os.Exit(0)

}

func printUsage(name string) {
	fmt.Printf("******************************************\n")
	fmt.Printf("USAGE:\n\t%v --script [TICKER] [ITERATIONS] // executes the routine every [TICKER] minutes per [ITERATIONS]\n", name)
	fmt.Printf("\t%v --dht // tries to reconstruct the DHT table of the Peer\n", name)
	fmt.Printf("DEFAULT VALUES:\n\tTICKER = %v\n\tITERATIONS = %v\n", routineInterval, maxExecutions)
}

func main() {
	args := os.Args

	if len(args) <= 1 || (len(args) > 1 && !(args[1] == "--script" || args[1] == "--dht")) {
		printUsage(args[0])
		os.Exit(0)
	}

	if args[1] == "--script" && enablePings {
		p, _ := ping.NewPinger("8.8.8.8")
		if !p.Privileged() {
			fmt.Printf("******************************************\n")
			fmt.Printf("If you need the ping statistics the application must run in privileged mode\n")
			fmt.Printf("Disabling pings and starting the application\n")
			fmt.Printf("******************************************\n")
			enablePings = false
		}
	}
	if args[1] == "--script" && len(args) != 4 {
		script()
	} else if args[1] == "--script" && len(args) == 4 {
		temp, err := time.ParseDuration(args[2])
		if err != nil {
			printUsage(args[0])
			os.Exit(0)
		}
		routineInterval = temp
		t, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			printUsage(args[0])
			os.Exit(0)
		}

		maxExecutions = int(t)
		script()
	} else {
		dht()
	}
}
