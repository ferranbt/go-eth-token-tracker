package postgresql

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ferranbt/go-eth-token-tracker/store"
	"github.com/gobuffalo/packr"
	"github.com/jmoiron/sqlx"
	"github.com/umbracle/go-web3"
	"github.com/umbracle/go-web3/abi"
	"github.com/umbracle/go-web3/contract/builtin/erc20"
)

const (
	defaultEndpoint = "user=postgres dbname=postgres sslmode=disable"
)

// Factory is the factory method for the Postgresql store
func Factory(config map[string]interface{}) (store.Store, error) {
	endpoint := defaultEndpoint
	
	endpointRaw, ok := config["endpoint"]
	if ok {
		endpoint, ok = endpointRaw.(string)
		if !ok {
			return nil, fmt.Errorf("cannot convert endpoint to string")
		}
	}
	return New(endpoint)
}

var (
	transferEvent = erc20.ERC20Abi().Events["Transfer"]
)

var (
	ddl = packr.NewBox("./db")
)

// Store is a PostgreSQL store for the tracker
type Store struct {
	db *sqlx.DB
}

// New creates a new store
func New(endpoint string) (*Store, error) {
	db, err := sqlx.Connect("postgres", endpoint)
	if err != nil {
		return nil, err
	}
	s := &Store{db}
	if err := s.setupDB(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) setupDB() error {
	var count int
	if err := s.db.Get(&count, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'"); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	if _, err := s.db.Exec(ddl.String("./schema/schema.sql")); err != nil {
		return err
	}
	return nil
}

// WriteReceipt writes a new receipt
func (s *Store) WriteReceipt(logs []*web3.Log) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	for _, log := range logs {
		if err := s.writeLogImpl(tx, log); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) writeLogImpl(tx *sqlx.Tx, log *web3.Log) error {
	if len(log.Topics) != 3 {
		// non-standard erc20 token
		return nil
	}
	vals, err := abi.ParseLog(transferEvent.Inputs, log)
	if err != nil {
		return err
	}

	token := log.Address
	if err := s.writeTokenImpl(tx, token); err != nil {
		return err
	}

	from, err := decodeAddress(vals, "from")
	if err != nil {
		return err
	}
	to, err := decodeAddress(vals, "to")
	if err != nil {
		return err
	}
	value, err := decodeBigInt(vals, "value")
	if err != nil {
		return err
	}
	query := "INSERT INTO transfers (token_id, block_hash, txn_hash, from_addr, to_addr, value) VALUES ($1, $2, $3, $4, $5, $6)"
	if _, err := tx.Exec(query, token.String(), log.BlockHash.String(), log.TransactionHash.String(), from.String(), to.String(), value.String()); err != nil {
		return err
	}
	return nil
}

func (s *Store) writeTokenImpl(tx *sqlx.Tx, token web3.Address) error {
	var count int
	if err := tx.Get(&count, "SELECT count(*) FROM tokens WHERE id=$1", token.String()); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	if _, err := tx.Exec("INSERT INTO tokens (id) VALUES ($1)", token.String()); err != nil {
		return err
	}
	return nil
}

// Close closes the storage
func (s *Store) Close() error {
	return s.db.Close()
}

// RemoveReceipts removes the receipts by block hash
func (s *Store) RemoveReceipts(blockHash web3.Hash) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}

	query := "DELETE FROM transfers WHERE block_hash=$1"
	if _, err := tx.Exec(query, blockHash.String()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func decodeAddress(vals map[string]interface{}, attr string) (web3.Address, error) {
	val, ok := vals[attr]
	if !ok {
		return web3.Address{}, fmt.Errorf("key '%s' not found", attr)
	}
	valStr, ok := val.(web3.Address)
	if !ok {
		fmt.Println(val)
		return web3.Address{}, fmt.Errorf("cannot convert '%s' to string", attr)
	}
	return valStr, nil
}

func decodeBigInt(vals map[string]interface{}, attr string) (*big.Int, error) {
	val, ok := vals[attr]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found", attr)
	}
	valStr, ok := val.(*big.Int)
	if !ok {
		return nil, fmt.Errorf("cannot convert '%s' to big.Int", attr)
	}
	return valStr, nil
}

// ListTokens returns the list of registered tokens
func (s *Store) ListTokens(p store.QueryPagination) ([]string, error) {
	var tokens []string
	query := "SELECT id FROM tokens" + p.String()
	if err := s.db.Select(&tokens, query); err != nil {
		return nil, err
	}
	return tokens, nil
}

func sliceAddressToString(w []web3.Address) []string {
	resp := []string{}
	for _, i := range w {
		resp = append(resp, i.String())
	}
	return resp
}

// GetTokenTransfers returns the transfers given a filter
func (s *Store) GetTokenTransfers(filter store.TransfersFilter) ([]*store.Transfer, error) {
	query := "SELECT token_id, from_addr, to_addr, value FROM transfers"

	whereAttr := []string{}
	// filter by from
	if len(filter.From) != 0 {
		whereAttr = append(whereAttr, "from_addr IN ('"+strings.Join(sliceAddressToString(filter.From), "', '")+"')")
	}
	// filter by to
	if len(filter.To) != 0 {
		whereAttr = append(whereAttr, "to_addr IN ('"+strings.Join(sliceAddressToString(filter.To), "', '")+"')")
	}
	// filter by tokens
	if len(filter.Tokens) != 0 {
		whereAttr = append(whereAttr, "token_id IN ('"+strings.Join(sliceAddressToString(filter.Tokens), "', '")+"')")
	}
	if len(whereAttr) != 0 {
		query += " WHERE " + strings.Join(whereAttr, " AND ")
	}

	// add the pagination
	query += filter.QueryPagination.String()

	transfers := []*store.Transfer{}
	if err := s.db.Select(&transfers, query); err != nil {
		return nil, err
	}
	return transfers, nil
}
