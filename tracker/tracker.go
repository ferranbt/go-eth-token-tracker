package tracker

import (
	"context"
	"log"

	"github.com/cheggaaa/pb/v3"
	"github.com/umbracle/go-web3"
	"github.com/umbracle/go-web3/jsonrpc"
	"github.com/umbracle/go-web3/tracker"
	trackerboltdb "github.com/umbracle/go-web3/tracker/boltdb"
)

var (
	transferEventTopic = web3.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
)

// Config is the configuration for the token tracker.
type Config struct {
	Endpoint    string `mapstructure:"endpoint"`
	BoltDBPath  string `mapstructure:"dbpath"`
	BatchSize   int64  `mapstructure:"batchsize"`
	ProgressBar bool   `mapstructure:"progressbar"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		BoltDBPath:  "data.db",
		BatchSize:   1000,
		ProgressBar: true,
		Endpoint:    "https://mainnet.infura.io",
	}
}

// Store is the storage interface required by the tracker
type Store interface {
	WriteReceipt(logs []*web3.Log) error
	RemoveReceipts(hash web3.Hash) error
	Close() error
}

// TokenTracker tracks ERC20 tokens
type TokenTracker struct {
	logger  *log.Logger
	store   Store
	config  *Config
	tracker *tracker.Tracker
	client  *jsonrpc.Client
	closeCh context.CancelFunc
}

// NewTokenTracker creates a new token tracker
func NewTokenTracker(logger *log.Logger, config *Config, store Store) (*TokenTracker, error) {
	t := &TokenTracker{
		logger: logger,
		config: config,
		store:  store,
	}

	client, err := jsonrpc.NewClient(config.Endpoint)
	if err != nil {
		return nil, err
	}
	t.client = client

	boltdbStore, err := trackerboltdb.New(config.BoltDBPath)
	if err != nil {
		return nil, err
	}

	trackerConfig := tracker.DefaultConfig()
	trackerConfig.BatchSize = uint64(config.BatchSize)
	t.tracker = tracker.NewTracker(client.Eth(), trackerConfig)
	t.tracker.SetStore(boltdbStore)

	// token Transfer event
	t.tracker.SetFilterTopics([]*web3.Hash{
		&transferEventTopic,
	})

	return t, nil
}

// Sync starts the tracker
func (t *TokenTracker) Sync(ctx context.Context) error {
	if t.config.ProgressBar {
		if err := t.startProgressBar(ctx); err != nil {
			return err
		}
	}

	eventCh := make(chan *tracker.Event, 1024)
	t.tracker.EventCh = eventCh

	ctx, cancel := context.WithCancel(ctx)
	t.closeCh = cancel

	var syncErr error
	handleErr := func(err error) {
		cancel()
		syncErr = err
	}

	go func() {
		for {
			select {
			case evnt := <-eventCh:
				for _, r := range evnt.RemovedLogs {
					if err := t.store.RemoveReceipts(r.BlockHash); err != nil {
						handleErr(err)
						return
					}
				}
				if len(evnt.AddedLogs) != 0 {
					if err := t.store.WriteReceipt(evnt.AddedLogs); err != nil {
						handleErr(err)
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	if err := t.tracker.Sync(ctx); err != nil {
		handleErr(err)
	}
	if ctx.Err() == nil {
		t.tracker.Polling(ctx)
	}
	return syncErr
}

func (t *TokenTracker) startProgressBar(ctx context.Context) error {
	lastKnownBlock, err := t.client.Eth().BlockNumber()
	if err != nil {
		return err
	}

	syncCh := make(chan uint64, 100)
	t.tracker.SyncCh = syncCh

	bar := pb.New(int(lastKnownBlock))
	bar.Start()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case n := <-syncCh:
				bar.SetCurrent(int64(n))
			}
		}
	}()

	return nil
}

// Stop stops the tracker
func (t *TokenTracker) Stop() {
	t.closeCh()
	t.store.Close()
}
