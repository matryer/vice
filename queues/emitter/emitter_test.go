package emitter

import (
	"testing"

	"github.com/matryer/vice/v2"
	"github.com/matryer/vice/v2/vicetest"
)

func TestTransport(t *testing.T) {
	f := func() vice.Transport { return New() }
	vicetest.Transport(t, f)
}
