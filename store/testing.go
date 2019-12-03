package store

import (
	"math/big"
	"testing"

	"github.com/umbracle/go-web3"
	"github.com/umbracle/go-web3/abi"
	"github.com/umbracle/go-web3/contract/builtin/erc20"
)

var (
	transferEvent = erc20.ERC20Abi().Events["Transfer"]
)

func encodeERC20(r *web3.Receipt, token, from, to web3.Address, balance *big.Int) *web3.Log {
	encodeTopic := func(t *abi.Argument, i interface{}) web3.Hash {
		hash, err := abi.EncodeTopic(t.Type, i)
		if err != nil {
			panic(err)
		}
		return hash
	}

	buf, err := abi.Encode(balance, transferEvent.Inputs[2].Type)
	if err != nil {
		panic(err)
	}

	log := &web3.Log{
		Address:         token,
		BlockHash:       r.BlockHash,
		TransactionHash: r.TransactionHash,
		Topics: []web3.Hash{
			transferEvent.ID(),
			encodeTopic(transferEvent.Inputs[0], from),
			encodeTopic(transferEvent.Inputs[1], to),
		},
		Data: buf,
	}
	return log
}

var (
	hash1 = web3.HexToHash("0x0000000000000000000000000000000000000001")
	hash2 = web3.HexToHash("0x0000000000000000000000000000000000000002")
)

var (
	addr1 = web3.HexToAddress("0x0000000000000000000000000000000000000001")
	addr2 = web3.HexToAddress("0x0000000000000000000000000000000000000002")
	addr3 = web3.HexToAddress("0x0000000000000000000000000000000000000003")
	addr4 = web3.HexToAddress("0x0000000000000000000000000000000000000004")
)

type testFunc func(t *testing.T) (Store, func())

func testWriteReceipts(t *testing.T, tt testFunc) {
	store, close := tt(t)
	defer close()

	r0 := &web3.Receipt{
		BlockHash:       hash1,
		TransactionHash: hash2,
	}
	logs := []*web3.Log{
		encodeERC20(r0, addr3, addr1, addr2, big.NewInt(1000)),
		encodeERC20(r0, addr4, addr2, addr1, big.NewInt(100)),
	}

	if err := store.WriteReceipt(logs); err != nil {
		t.Fatal(err)
	}

	tokens, err := store.ListTokens(QueryPagination{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 {
		t.Fatal("2 tokens expected")
	}

	transfers, err := store.GetTokenTransfers(TransfersFilter{Tokens: []web3.Address{addr3, addr4}})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 2 {
		t.Fatal("2 transfers expected")
	}

	if err := store.RemoveReceipts(hash1); err != nil {
		t.Fatal(err)
	}

	transfers, err = store.GetTokenTransfers(TransfersFilter{Tokens: []web3.Address{addr3, addr4}})
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) == 0 {
		t.Fatal("no transfers expected")
	}
}

// TestStore is a generic test function to test different storage methods
func TestStore(t *testing.T, tt testFunc) {
	testWriteReceipts(t, tt)
}
