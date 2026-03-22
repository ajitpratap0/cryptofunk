package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "-1", itoa(-1))
	assert.Equal(t, "100", itoa(100))
}
