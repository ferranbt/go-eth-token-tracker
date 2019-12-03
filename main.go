package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/signal"
	"time"

	"os"
	"syscall"

	"github.com/hashicorp/hcl"
	"github.com/imdario/mergo"

	"github.com/ferranbt/go-eth-token-tracker/store/postgresql"

	"github.com/ferranbt/go-eth-token-tracker/http"
	"github.com/ferranbt/go-eth-token-tracker/store"
	"github.com/ferranbt/go-eth-token-tracker/tracker"
	tokentracker "github.com/ferranbt/go-eth-token-tracker/tracker"

	_ "github.com/lib/pq"
)

// Config is the generic config of the tracker
type Config struct {
	HTTP    *http.Config           `mapstructure:"http"`
	Tracker *tracker.Config        `mapstructure:"tracker"`
	Storage map[string]interface{} `mapstructure:"storage"`
}

func (c *Config) merge(c1 ...*Config) error {
	for _, i := range c1 {
		if err := mergo.Merge(c, *i, mergo.WithOverride); err != nil {
			return err
		}
	}
	return nil
}

func defaultConfig() *Config {
	return &Config{
		HTTP:    http.DefaultConfig(),
		Tracker: tracker.DefaultConfig(),
		Storage: map[string]interface{}{},
	}
}

func builtConfig() (*Config, error) {
	config := defaultConfig()

	cliConfig := &Config{
		HTTP:    &http.Config{},
		Tracker: &tracker.Config{},
		Storage: map[string]interface{}{},
	}

	var configPath, dbEndpoint string

	flag.StringVar(&cliConfig.HTTP.Addr, "http-addr", "", "")
	flag.StringVar(&cliConfig.Tracker.Endpoint, "jsonrpc-endpoint", "", "")
	flag.StringVar(&cliConfig.Tracker.BoltDBPath, "boltdb-path", "", "")
	flag.StringVar(&dbEndpoint, "db-endpoint", "", "")
	flag.Int64Var(&cliConfig.Tracker.BatchSize, "batch-size", 0, "")
	flag.BoolVar(&cliConfig.Tracker.ProgressBar, "progress-bar", false, "")
	flag.StringVar(&configPath, "config", "", "")

	flag.Parse()

	if dbEndpoint != "" {
		cliConfig.Storage["endpoint"] = dbEndpoint
	}

	if configPath != "" {
		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		configFile := &Config{}
		if err := hcl.Decode(configFile, string(data)); err != nil {
			return nil, err
		}
		if err := config.merge(configFile); err != nil {
			return nil, err
		}
	}
	if err := config.merge(cliConfig); err != nil {
		return nil, err
	}
	return config, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("[ERROR]: %v", err)
		os.Exit(1)
	}
}

func run() error {
	config, err := builtConfig()
	if err != nil {
		return fmt.Errorf("failed to read config %v", err)
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)

	storageFactory := builtin["postgresql"]
	store, err := storageFactory(config.Storage)
	if err != nil {
		return fmt.Errorf("failed to build storage: %v", err)
	}

	httpServer, err := http.NewServer(logger, config.HTTP, store)
	if err != nil {
		return fmt.Errorf("failed to build http server: %v", err)
	}

	tracker, err := tokentracker.NewTokenTracker(logger, config.Tracker, store)
	if err != nil {
		return fmt.Errorf("failed to start tracker: %v", err)
	}

	go func() {
		tracker.Sync(context.Background())
	}()

	close := func() {
		httpServer.Stop()
		tracker.Stop()
	}
	handleSignals(close)
	return nil
}

func handleSignals(cancelFn func()) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case <-signalCh:
	}

	gracefulCh := make(chan struct{})
	go func() {
		cancelFn()
		close(gracefulCh)
	}()

	select {
	case <-signalCh:
		return 1
	case <-time.After(10 * time.Second):
		return 1
	case <-gracefulCh:
		return 0
	}
}

// Factory is the factory method for the database
type Factory func(config map[string]interface{}) (store.Store, error)

var builtin map[string]Factory

func register(name string, f Factory) {
	if len(builtin) == 0 {
		builtin = map[string]Factory{}
	}
	builtin[name] = f
}

func init() {
	register("postgresql", postgresql.Factory)
}
