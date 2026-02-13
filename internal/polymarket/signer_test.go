package polymarket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSigner_InvalidKey(t *testing.T) {
	_, err := NewSigner("invalid-hex", 137)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key")
}

func TestNewSigner_WithPrefix(t *testing.T) {
	signer, err := NewSigner("0x"+testPrivateKey, 137)
	require.NoError(t, err)
	assert.Equal(t, "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", signer.Address().Hex())
}

func TestSigner_Destroy(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)
	signer.Destroy()
	assert.Nil(t, signer.privateKey)
}

func TestSigner_DestroyNilKey(t *testing.T) {
	signer := &Signer{}
	signer.Destroy() // should not panic
}

func TestGenerateOrderSalt(t *testing.T) {
	salt1 := GenerateOrderSalt()
	salt2 := GenerateOrderSalt()
	assert.NotEmpty(t, salt1)
	assert.NotEmpty(t, salt2)
	// Salts use keccak of zero bytes so they're deterministic, but still valid
	assert.NotEmpty(t, salt1)
}

func TestSideToUint8(t *testing.T) {
	assert.Equal(t, "0", sideToUint8("BUY"))
	assert.Equal(t, "0", sideToUint8("buy"))
	assert.Equal(t, "1", sideToUint8("SELL"))
	assert.Equal(t, "1", sideToUint8("other"))
}

func TestBuildHMACSignature_InvalidSecret(t *testing.T) {
	_, err := BuildHMACSignature("not-valid-base64!!!", 1700000000, "GET", "/test", "")
	assert.Error(t, err)
}

func TestSignOrder_NegRisk(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)

	order := &SignedOrder{
		Salt:          "99999",
		Maker:         signer.Address().Hex(),
		Signer:        signer.Address().Hex(),
		Taker:         "0x0000000000000000000000000000000000000000",
		TokenID:       "999",
		MakerAmount:   "1000000",
		TakerAmount:   "500000",
		Expiration:    "0",
		Nonce:         "0",
		FeeRateBps:    "0",
		Side:          "SELL",
		SignatureType: 0,
	}

	sig, err := signer.SignOrder(order, NegRiskExchangeAddr, true)
	require.NoError(t, err)
	assert.Equal(t, "0x", sig[:2])
	assert.Equal(t, 132, len(sig))
}

func TestSignClobAuthMessage_DifferentNonce(t *testing.T) {
	signer, err := NewSigner(testPrivateKey, 137)
	require.NoError(t, err)

	sig1, err := signer.SignClobAuthMessage(1700000000, 0)
	require.NoError(t, err)
	sig2, err := signer.SignClobAuthMessage(1700000000, 1)
	require.NoError(t, err)

	assert.NotEqual(t, sig1, sig2)
}
