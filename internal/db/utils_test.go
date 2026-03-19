package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPtrFloat64(t *testing.T) {
	v := PtrFloat64(3.14)
	require.NotNil(t, v)
	assert.Equal(t, 3.14, *v)

	zero := PtrFloat64(0.0)
	require.NotNil(t, zero)
	assert.Equal(t, 0.0, *zero)

	neg := PtrFloat64(-99.5)
	require.NotNil(t, neg)
	assert.Equal(t, -99.5, *neg)
}
