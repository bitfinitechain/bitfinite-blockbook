package bitfinite

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
)

// BitfiniteRPC is an interface to JSON-RPC bitfinited service.
type BitfiniteRPC struct {
	*btc.BitcoinRPC
}

// NewBitfiniteRPC returns new BitfiniteRPC instance.
func NewBitfiniteRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &BitfiniteRPC{
		b.(*btc.BitcoinRPC),
	}
	s.ChainConfig.SupportsEstimateSmartFee = false

	return s, nil
}

// Initialize initializes BitfiniteRPC instance.
func (b *BitfiniteRPC) Initialize() error {
	ci, err := b.GetChainInfo()
	if err != nil {
		return err
	}
	chainName := ci.Chain

	params := GetChainParams(chainName)

	// always create parser
	b.Parser, err = NewBitfiniteParser(params, b.ChainConfig)
	if err != nil {
		return err
	}

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)

	return nil
}

// getblock

type cmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbose   bool   `json:"verbose"`
	} `json:"params"`
}

// GetBlock returns block with given hash.
func (b *BitfiniteRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" && height > 0 {
		hash, err = b.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
	}
	header, err := b.GetBlockHeader(hash)
	if err != nil {
		return nil, err
	}
	data, err := b.GetBlockBytes(hash)
	if err != nil {
		return nil, err
	}
	block, err := b.Parser.ParseBlock(data)
	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	size := block.Size
	block.BlockHeader = *header
	block.Size = size
	return block, nil
}

// GetBlockRaw returns block with given hash as bytes.
func (b *BitfiniteRPC) GetBlockRaw(hash string) (string, error) {
	glog.V(1).Info("rpc: getblock (verbose=0) ", hash)

	res := btc.ResGetBlockRaw{}
	req := cmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = false
	err := b.Call(&req, &res)

	if err != nil {
		return "", errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return "", bchain.ErrBlockNotFound
		}
		return "", errors.Annotatef(res.Error, "hash %v", hash)
	}
	return res.Result, nil
}

// GetBlockBytes returns block with given hash as bytes
func (b *BitfiniteRPC) GetBlockBytes(hash string) ([]byte, error) {
	block, err := b.GetBlockRaw(hash)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(block)
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *BitfiniteRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

	res := btc.ResGetBlockInfo{}
	req := cmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = true
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if isErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}
	return &res.Result, nil
}

// GetBlockFull returns block with given hash.
func (b *BitfiniteRPC) GetBlockFull(hash string) (*bchain.Block, error) {
	return nil, errors.New("Not implemented")
}

func isErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block height out of range"
}

// EstimateFee returns fee estimation
func (b *BitfiniteRPC) EstimateFee(blocks int) (big.Int, error) {
	glog.V(1).Info("rpc: estimatefee ", blocks)

	res := btc.ResEstimateFee{}
	req := struct {
		Method string `json:"method"`
	}{
		Method: "estimatefee",
	}

	err := b.Call(&req, &res)

	var r big.Int
	if err != nil {
		return r, err
	}
	if res.Error != nil {
		return r, res.Error
	}
	r, err = b.Parser.AmountToBigInt(res.Result)
	if err != nil {
		return r, err
	}
	return r, nil
}

// EstimateSmartFee returns fee estimation
func (b *BitfiniteRPC) EstimateSmartFee(blocks int, conservative bool) (big.Int, error) {
	return b.EstimateFee(blocks)
}
