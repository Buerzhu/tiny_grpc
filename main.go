package main

import (
	"flag"

	log "github.com/golang/glog"
)

func main() {
	flag.Parse()
	defer log.Flush()
}
