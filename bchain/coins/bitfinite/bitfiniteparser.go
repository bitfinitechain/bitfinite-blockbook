package bitfinite

import (
	"fmt"

	"github.com/martinboehm/bchutil"
	"github.com/martinboehm/btcd/wire"
	"github.com/martinboehm/btcutil"
	"github.com/martinboehm/btcutil/chaincfg"
	"github.com/martinboehm/btcutil/txscript"
	"github.com/schancel/cashaddr-converter/address"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
)

// AddressFormat type is used to specify different formats of address
type AddressFormat = uint8

const (
	// Legacy AddressFormat is the same as Bitcoin
	Legacy AddressFormat = iota
	// CashAddr AddressFormat is the BitFinite (Bitcoin Cash style) standard
	CashAddr
)

const (
	// MainNetPrefix is the CashAddr prefix for mainnet
	MainNetPrefix = "bfx:"
	// TestNetPrefix is the CashAddr prefix for testnet
	TestNetPrefix = "bfxtest:"
	// RegTestPrefix is the CashAddr prefix for regtest
	RegTestPrefix = "bfxreg:"
)

// BitFinite network magic (chainparams netMagic 42 46 69 6e → LE uint32).
const (
	MainnetMagic wire.BitcoinNet = 0x6e694642
	TestnetMagic wire.BitcoinNet = 0x6e694644
	Regtestmagic wire.BitcoinNet = 0x6e694645
)

var (
	// MainNetParams are parser parameters for mainnet
	MainNetParams chaincfg.Params
	// TestNetParams are parser parameters for testnet
	TestNetParams chaincfg.Params
	// RegtestParams are parser parameters for regtest
	RegtestParams chaincfg.Params
)

func init() {
	// BitFinite uses standard Bitcoin base58 + BIP32 (xpub) prefixes, so we start
	// from Bitcoin's params and only override the network magic + a distinct name
	// (the name keys the cashaddr prefix lookup in the patched bchutil).
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Name = "bitfinite"
	MainNetParams.Net = MainnetMagic

	TestNetParams = chaincfg.TestNet3Params
	TestNetParams.Name = "bitfinite-test"
	TestNetParams.Net = TestnetMagic

	RegtestParams = chaincfg.RegressionNetParams
	RegtestParams.Name = "bitfinite-regtest"
	RegtestParams.Net = Regtestmagic
}

// BitfiniteParser handle
type BitfiniteParser struct {
	*btc.BitcoinLikeParser
	AddressFormat AddressFormat
}

// NewBitfiniteParser returns new BitfiniteParser instance
func NewBitfiniteParser(params *chaincfg.Params, c *btc.Configuration) (*BitfiniteParser, error) {
	var format AddressFormat
	switch c.AddressFormat {
	case "":
		fallthrough
	case "cashaddr":
		format = CashAddr
	case "legacy":
		format = Legacy
	default:
		return nil, fmt.Errorf("Unknown address format: %s", c.AddressFormat)
	}
	p := &BitfiniteParser{
		BitcoinLikeParser: btc.NewBitcoinLikeParser(params, c),
		AddressFormat:     format,
	}
	p.OutputScriptToAddressesFunc = p.outputScriptToAddresses
	return p, nil
}

// GetChainParams contains network parameters for the main, testnet and regtest
// BitFinite networks, in this order
func GetChainParams(chain string) *chaincfg.Params {
	if !chaincfg.IsRegistered(&MainNetParams) {
		err := chaincfg.Register(&MainNetParams)
		if err == nil {
			err = chaincfg.Register(&TestNetParams)
		}
		if err == nil {
			err = chaincfg.Register(&RegtestParams)
		}
		if err != nil {
			panic(err)
		}
	}
	switch chain {
	case "test":
		return &TestNetParams
	case "regtest":
		return &RegtestParams
	default:
		return &MainNetParams
	}
}

// GetAddrDescFromAddress returns internal address representation of given address
func (p *BitfiniteParser) GetAddrDescFromAddress(address string) (bchain.AddressDescriptor, error) {
	return p.addressToOutputScript(address)
}

// addressToOutputScript converts address to ScriptPubKey
func (p *BitfiniteParser) addressToOutputScript(address string) ([]byte, error) {
	if isCashAddr(address) {
		da, err := bchutil.DecodeAddress(address, p.Params)
		if err != nil {
			return nil, err
		}
		script, err := bchutil.PayToAddrScript(da)
		if err != nil {
			return nil, err
		}
		return script, nil
	}
	da, err := btcutil.DecodeAddress(address, p.Params)
	if err != nil {
		return nil, err
	}
	script, err := txscript.PayToAddrScript(da)
	if err != nil {
		return nil, err
	}
	return script, nil
}

func isCashAddr(addr string) bool {
	n := len(addr)
	switch {
	case n > len(MainNetPrefix) && addr[0:len(MainNetPrefix)] == MainNetPrefix:
		return true
	case n > len(TestNetPrefix) && addr[0:len(TestNetPrefix)] == TestNetPrefix:
		return true
	case n > len(RegTestPrefix) && addr[0:len(RegTestPrefix)] == RegTestPrefix:
		return true
	}
	return false
}

// outputScriptToAddresses converts ScriptPubKey to addresses
func (p *BitfiniteParser) outputScriptToAddresses(script []byte) ([]string, bool, error) {
	var err error
	script, err = txscript.ConvertP2PKtoP2PKH(p.Params.Base58CksumHasher, script)
	if err != nil {
		return nil, false, err
	}
	a, err := bchutil.ExtractPkScriptAddrs(script, p.Params)
	if err != nil {
		if err.Error() == "unknown script type" {
			or := p.TryParseOPReturn(script)
			if or != "" {
				return []string{or}, false, nil
			}
			return []string{}, false, nil
		}
		return nil, false, err
	}
	addr := a.EncodeAddress()
	if p.AddressFormat == Legacy {
		da, err := address.NewFromString(addr)
		if err != nil {
			return nil, false, err
		}
		ca, err := da.Legacy()
		if err != nil {
			return nil, false, err
		}
		addr, err = ca.Encode()
		if err != nil {
			return nil, false, err
		}
	}
	return []string{addr}, len(addr) > 0, nil
}
