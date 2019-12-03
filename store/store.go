package store

import (
	"fmt"

	"github.com/umbracle/go-web3"
)

// Transfer is the model for a token transfer
type Transfer struct {
	Addr  string `db:"token_id"`
	From  string `db:"from_addr"`
	To    string `db:"to_addr"`
	Value string `db:"value"`
}

// QueryPagination represents a database pagination query
type QueryPagination struct {
	Limit  int
	Offset int
}

func (q *QueryPagination) String() string {
	if q.Limit == 0 {
		return ""
	}
	return fmt.Sprintf(" LIMIT %d OFFSET %d", q.Limit, q.Offset)
}

func sliceAddressToString(w []web3.Address) []string {
	resp := []string{}
	for _, i := range w {
		resp = append(resp, i.String())
	}
	return resp
}

// TransfersFilter is the filter for a token transfer
type TransfersFilter struct {
	QueryPagination

	From   []web3.Address
	To     []web3.Address
	Tokens []web3.Address
}

// Store is the interface to access the store
type Store interface {
	WriteReceipt(logs []*web3.Log) error
	RemoveReceipts(blockHash web3.Hash) error
	Close() error
	ListTokens(p QueryPagination) ([]string, error)
	GetTokenTransfers(filter TransfersFilter) ([]*Transfer, error)
}
