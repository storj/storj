package planet

import (
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
)

type Satellite struct {
	Identity  *provider.FullIdentity
	Kademlia  *kademlia.Kademlia
	PointerDB pointerdb.Config
	Overlay   overlay.Config
	Checker   checker.Config
	Repairer  repairer.Config
}
