package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"
)

func main() {
	mainArgs(os.Args[1:])
}

func mainArgs(args []string) {
	var (
		configFile string
		listen     string
	)

	flags := flag.NewFlagSet("flux-recv", flag.ExitOnError)

	flags.StringVar(&configFile, "config", "fluxrecv.yaml", "path to config file for flux-recv") // TODO(michael): `flux-recv help config`
	flags.StringVar(&listen, "listen", ":8080", "address to listen on")

	bail := func(msg string) {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}

	flags.Parse(args)

	config, err := ConfigFromFile(configFile)
	if err != nil {
		bail(err.Error())
	}

	configDir := filepath.Dir(configFile)
	for _, ep := range config.Endpoints {
		println("endpoint", ep.Source, "using key", filepath.Join(configDir, ep.KeyPath))
	}

	http.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
		return
	})

	http.ListenAndServe(listen, nil)
}
