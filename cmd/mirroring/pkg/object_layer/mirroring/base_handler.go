package mirroring

import (
	"context"
)

// Base handler with all common fields
type baseHandler struct {
	primeErr, alterErr error
	ctx context.Context
	m *MirroringObjectLayer
}
