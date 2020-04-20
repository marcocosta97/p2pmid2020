package main

import "time"

// This module provides the configuration options for the application
//
// Author: Marco Costa

// The IPFS HTTP API address, needed for contacting the IPFS daemon instance
const ipfsAPI string = "localhost:5001"

// The API token for https://ipinfo.io/ is needed for the country IP lookup
const apiToken string = "2ea5eaba35e3dd"

// Default parameters for the routine interval and the max number of executions
// can be overwritten using command line arguments
// @see usage
var routineInterval time.Duration = 1 * time.Hour
var maxExecutions int = 72

// If this is true and the script is run in privileged mode the pings statistics
// are enabled
var enablePings bool = true

// If this is true the RTT for every known IP is saved and not recalculated in case
// of future uses
const cachePings bool = true

// Default number of ping iterations and timeout
// higher values give a more accurate response
const pingIterations int = 1
const pingTimeout = 1 * time.Second

// Default filename for the json output
// Format: HHHH-MM-DD HH-MM.json
var dataFilename string = time.Now().Format("2006-01-02 15-04") + ".json"

// Default format for the timestamps
// Day: HHHH-MM-DD
// Time: HH:MM:SS
const jsonDayFormat string = "2006-01-02"
const jsonHourFormat string = "15:04:05"

// The format of the RTT statistics
const rttFormat int64 = int64(time.Millisecond)
