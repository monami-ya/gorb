/*
   Copyright (c) 2015 Andrey Sibiryov <me@kobology.ru>
   Copyright (c) 2015 Other contributors as noted in the AUTHORS file.

   This file is part of GORB - Go Routing and Balancing.

   GORB is free software; you can redistribute it and/or modify
   it under the terms of the GNU Lesser General Public License as published by
   the Free Software Foundation; either version 3 of the License, or
   (at your option) any later version.

   GORB is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
   GNU Lesser General Public License for more details.

   You should have received a copy of the GNU Lesser General Public License
   along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kobolog/gorb/core"
	"github.com/kobolog/gorb/util"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	debug  = flag.Bool("v", false, "enable verbose output")
	device = flag.String("i", "eth0", "default interface to bind services on")
	flush  = flag.Bool("f", false, "flush IPVS pools on start")
	listen = flag.String("l", ":4672", "endpoint to listen for HTTP requests")
	consul = flag.String("c", "", "URL for Consul HTTP API")
)

func ping() {
	time.Sleep(2 * time.Second)

	log.Info("Start multicasting packet to Gorb-links.");

	addr, err := net.ResolveUDPAddr("udp", "224.0.0.1" + *listen)
	if err != nil {
		log.Errorf("Error in parse UDPAddr.")
		return
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Errorf(fmt.Sprintf("Error in DialUDP(): %s", err))
		return
	}
	_, err = conn.Write([]byte("ping!"))
	if err != nil {
		log.Errorf(fmt.Sprintf("Error in sending ping packet: %s", err))
	} else {
		log.Info("Ping multicast packet was sent.")
	}
}

func main() {
	// Called first to interrupt bootstrap and display usage if the user passed -h.
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Info("starting GORB Daemon v0.2")

	if os.Geteuid() != 0 {
		log.Fatalf("this program has to be run with root priveleges to access IPVS")
	}

	hostIPs, err := util.InterfaceIPs(*device)
	if err != nil {
		log.Fatalf("error while obtaining interface addresses: %s", err)
	}

	ctx, err := core.NewContext(core.ContextOptions{
		Disco:     *consul,
		Endpoints: hostIPs,
		Flush:     *flush})

	if err != nil {
		log.Fatalf("error while initializing server context: %s", err)
	}

	// While it's not strictly required, close IPVS socket explicitly.
	defer ctx.Close()

	r := mux.NewRouter()

	r.Handle("/service/{vsID}", serviceCreateHandler{ctx}).Methods("PUT")
	r.Handle("/service/{vsID}/{rsID}", backendCreateHandler{ctx}).Methods("PUT")
	r.Handle("/service/{vsID}/{rsID}", backendUpdateHandler{ctx}).Methods("PATCH")
	r.Handle("/service/{vsID}", serviceRemoveHandler{ctx}).Methods("DELETE")
	r.Handle("/service/{vsID}/{rsID}", backendRemoveHandler{ctx}).Methods("DELETE")
	r.Handle("/service/{vsID}", serviceStatusHandler{ctx}).Methods("GET")
	r.Handle("/service/{vsID}/{rsID}", backendStatusHandler{ctx}).Methods("GET")

	if (*consul == "") {
		go ping()
	}

	log.Infof("setting up HTTP server on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, r))
}
