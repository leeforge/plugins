package factory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEntFactory_NotNil(t *testing.T) {
	f := NewEntFactory(nil)
	require.NotNil(t, f)
}
