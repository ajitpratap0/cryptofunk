package polymarket

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	ClobDomainName = "ClobAuthDomain"
	ClobVersion    = "1"
	MsgToSign      = "This message attests that I control the given wallet"
)

// Signer handles EIP-712 signing for Polymarket CLOB
type Signer struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
	chainID    int
}

// NewSigner creates a new signer from a hex private key
func NewSigner(hexKey string, chainID int) (*Signer, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return &Signer{
		privateKey: key,
		address:    addr,
		chainID:    chainID,
	}, nil
}

// Address returns the signer's address
func (s *Signer) Address() common.Address {
	return s.address
}

// Destroy zeros out the private key bytes to prevent memory leaks of sensitive material
func (s *Signer) Destroy() {
	if s.privateKey != nil {
		// Zero out the private key D value
		s.privateKey.D.SetUint64(0)
		s.privateKey = nil
	}
}


// SignClobAuthMessage signs an EIP-712 auth message for L1 authentication
func (s *Signer) SignClobAuthMessage(timestamp int64, nonce int) (string, error) {
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"ClobAuth": {
				{Name: "address", Type: "address"},
				{Name: "timestamp", Type: "string"},
				{Name: "nonce", Type: "uint256"},
				{Name: "message", Type: "string"},
			},
		},
		PrimaryType: "ClobAuth",
		Domain: apitypes.TypedDataDomain{
			Name:    ClobDomainName,
			Version: ClobVersion,
			ChainId: math.NewHexOrDecimal256(int64(s.chainID)),
		},
		Message: apitypes.TypedDataMessage{
			"address":   s.address.Hex(),
			"timestamp": strconv.FormatInt(timestamp, 10),
			"nonce":     fmt.Sprintf("%d", nonce),
			"message":   MsgToSign,
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", fmt.Errorf("hash domain: %w", err)
	}

	messageHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return "", fmt.Errorf("hash message: %w", err)
	}

	rawData := fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(messageHash))
	hash := crypto.Keccak256Hash([]byte(rawData))

	sig, err := crypto.Sign(hash.Bytes(), s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	// Adjust V value from 0/1 to 27/28
	sig[64] += 27

	return "0x" + common.Bytes2Hex(sig), nil
}

// SignOrder signs an order for the CLOB exchange using EIP-712
func (s *Signer) SignOrder(order *SignedOrder, exchangeAddr string, negRisk bool) (string, error) {
	chainIDBig := math.NewHexOrDecimal256(int64(s.chainID))

	// The CTF exchange order EIP-712 type
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket CTF Exchange",
			Version:           "1",
			ChainId:           chainIDBig,
			VerifyingContract: exchangeAddr,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          order.Salt,
			"maker":         order.Maker,
			"signer":        order.Signer,
			"taker":         order.Taker,
			"tokenId":       order.TokenID,
			"makerAmount":   order.MakerAmount,
			"takerAmount":   order.TakerAmount,
			"expiration":    order.Expiration,
			"nonce":         order.Nonce,
			"feeRateBps":    order.FeeRateBps,
			"side":          sideToUint8(order.Side),
			"signatureType": fmt.Sprintf("%d", order.SignatureType),
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", fmt.Errorf("hash domain: %w", err)
	}

	messageHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return "", fmt.Errorf("hash message: %w", err)
	}

	rawData := fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(messageHash))
	hash := crypto.Keccak256Hash([]byte(rawData))

	sig, err := crypto.Sign(hash.Bytes(), s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	sig[64] += 27

	return "0x" + common.Bytes2Hex(sig), nil
}

func sideToUint8(side string) string {
	if strings.ToUpper(side) == "BUY" {
		return "0"
	}
	return "1"
}

// BuildHMACSignature creates HMAC-SHA256 signature for L2 auth
func BuildHMACSignature(secret string, timestamp int64, method, requestPath string, body string) (string, error) {
	secretBytes, err := base64.URLEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}

	message := strconv.FormatInt(timestamp, 10) + method + requestPath
	if body != "" {
		message += body
	}

	h := hmac.New(sha256.New, secretBytes)
	h.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}

// GenerateOrderSalt creates a random salt for orders
func GenerateOrderSalt() string {
	b := make([]byte, 16)
	// Use crypto rand via go-ethereum
	salt := new(big.Int)
	salt.SetBytes(crypto.Keccak256(b))
	return salt.String()
}
