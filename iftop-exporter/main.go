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
	continuous := fs.Bool("continuous", false, "continuous mode")
	interval := fs.Duration("interval", 10*time.Second, "interval between two iftop runs, and must not be less than 10 seconds")
	duration := fs.Duration("duration", 3*time.Second,
		"duration of each iftop run, and must not be less than 3 seconds, and duration must be less than interval")
	version := fs.Bool("version", false, "print version")
	debug := fs.Bool("debug", false, "debug mode")
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
	if err != nil {
		log.Printf("create iftop manager failed, err: %s", err)
		os.Exit(1)
	}

	iftopManager.WithDebug(*debug)
	log.Printf("iftop execution pattern: continuous=%t, interval=%s, duration=%s", *continuous, *interval, *duration)

	if *continuous {
		log.Printf("WARN: continuous mode enabled, this mode may cause high CPU usage")
	} else {
		if *interval < 10*time.Second {
			log.Printf("Err: interval (%s) must not be less than 10 seconds", *interval)
			os.Exit(1)
		}

		if *duration < 3*time.Second {
			log.Printf("Err: duration (%s) must not be less than 3 seconds", *duration)
			os.Exit(1)
		}

		if *duration >= *interval {
			log.Printf("Err: duration (%s) must be less than interval (%s)", *duration, *interval)
			os.Exit(1)
		}
	}

	iftopManager.WithContinuous(*continuous, *interval, *duration)
	go iftopManager.Run()

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(*addr, nil); err != nil {
		fmt.Println(err)
	}
}
