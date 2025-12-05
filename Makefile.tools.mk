##@ Tooling

.PHONY: archview/svg
archview/svg: ## Create SVG diagrams using archview
	archview -root "storj.io/storj/satellite.Core"     -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-core.svg
	archview -root "storj.io/storj/satellite.API"      -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-api.svg
	archview -root "storj.io/storj/satellite.Repairer" -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ ./satellite/... | dot -T svg -o satellite-repair.svg
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   ./satellite/...   | dot -T svg -o satellite.svg
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ ./storagenode/... | dot -T svg -o storage-node.svg

.PHONY: archview/graphml
archview/graphml: ## Create graphml diagrams using archview
	archview -root "storj.io/storj/satellite.Core"     -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-core.graphml   ./satellite/...
	archview -root "storj.io/storj/satellite.API"      -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-api.graphml    ./satellite/...
	archview -root "storj.io/storj/satellite.Repairer" -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/ -out satellite-repair.graphml ./satellite/...
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/satellite/   -out satellite.graphml    ./satellite/...
	archview -skip-class "Peer,Master Database" -trim-prefix storj.io/storj/storagenode/ -out storage-node.graphml ./storagenode/...
