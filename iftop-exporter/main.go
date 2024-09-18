package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"log"

	"github.com/bougou/iftop-exporter/iftop-exporter/pkg/manager"
	pkgVersion "github.com/bougou/iftop-exporter/iftop-exporter/pkg/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	fs := flag.NewFlagSet("iftop-exporter", flag.ExitOnError)
	addr := fs.String("addr", ":9999", "Address to listen on")
	interfaces := fs.String("interfaces", "", "interface names separated by comma")
	dynamic := fs.Bool("dynamic", false, "dynamic mode")
	dynamicDir := fs.String("dynamic-dir", "/var/lib/iftop-exporter/dynamic", "dynamic directory")
	periodic := fs.Bool("periodic", false, "periodic mode")
	periodicInterval := fs.Duration("periodic-interval", 10*time.Second, "periodic mode interval, and must not be less than 10 seconds")
	periodicDuration := fs.Duration("periodic-duration", 3*time.Second,
		"periodic mode duration, and must not be less than 3 seconds, and periodicDuration must be less than periodicInterval")
	version := fs.Bool("version", false, "print version")
	help := fs.Bool("help", false, "print help")

	// above flag.ExitOnError makes sure the program exit when Parse failed.
	fs.Parse(os.Args[1:])

	if *help {
		fs.Usage()
		os.Exit(0)
	}

	if *version {
		fmt.Printf("Version: %s\n", pkgVersion.Version)
		fmt.Printf("Commit: %s\n", pkgVersion.Commit)
		fmt.Printf("BuildAt: %s\n", pkgVersion.BuildAt)
		os.Exit(0)
	}

	fmt.Println("args:", os.Args[1:])

	if !*dynamic && *interfaces == "" {
		log.Printf("the -dynamic and/or -interfaces option must be specified")
		os.Exit(1)
	}

	interfaceNames := []string{}
	if *interfaces != "" {
		for _, name := range strings.Split(*interfaces, ",") {
			n := strings.TrimSpace(name)
			if n != "" {
				interfaceNames = append(interfaceNames, n)
			}
		}
	}
	log.Printf("got (%d) static interfaces", len(interfaceNames))

	iftopManager, err := manager.NewManager(interfaceNames, *dynamic, *dynamicDir)
	if *periodic {
		log.Printf("periodic mode enabled")

		if *periodicInterval < 10*time.Second {
			log.Printf("Err: periodic interval (%s) must not be less than 10 seconds", *periodicInterval)
			os.Exit(1)
		}

		if *periodicDuration < 3*time.Second {
			log.Printf("Err: periodic duration (%s) must not be less than 3 seconds", *periodicDuration)
			os.Exit(1)
		}

		if *periodicDuration >= *periodicInterval {
			log.Printf("Err: periodic duration (%s) must be less than interval (%s)", *periodicDuration, *periodicInterval)
			os.Exit(1)
		}

		iftopManager.WithRunPeriodically(*periodicInterval, *periodicDuration)
	}

	if err != nil {
		log.Printf("create iftop manager failed, err: %s", err)
		os.Exit(1)
	}
	go iftopManager.Run()

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Println(err)
	}
}
