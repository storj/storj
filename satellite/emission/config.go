// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

// Config contains configurable values for emission service.
type Config struct {
	WriteEnergy                    float64 `help:"energy needed to write 1GB of data, in W-hours/GB" default:"0.005"`
	CO2PerEnergy                   float64 `help:"amount of carbon emission per unit of energy, in kg/kW-hours" default:"0.2826"`
	ShortenedDriveLife             float64 `help:"shortened hard drive life period, in years" default:"3"`
	StandardDriveLife              float64 `help:"standard hard drive life period, in years" default:"4"`
	ExtendedDriveLife              float64 `help:"extended hard drive life period, in years" default:"6"`
	NewDriveEmbodiedCarbon         float64 `help:"carbon footprint of producing 1TB HDD, in kg/TB" default:"20"`
	CarbonFromDrivePowering        float64 `help:"carbon from power per year of operations, in kg/TB-year" default:"15.9"`
	RepairedData                   float64 `help:"amount of repaired data, in TB" default:"667"`
	ExpandedData                   float64 `help:"amount of expanded data, in TB" default:"48689"`
	StorjGCPCarbon                 float64 `help:"amount of carbon emission from storj GCP, in kg" default:"3600"`
	StorjCRDBCarbon                float64 `help:"amount of carbon emission from storj CRDB, in kg" default:"2650"`
	StorjEdgeCarbon                float64 `help:"amount of carbon emission from storj Edge, in kg" default:"10924"`
	StorjExpandedNetworkStorage    float64 `help:"amount of expanded network storage, in TB" default:"18933"`
	HyperscalerExpansionFactor     float64 `help:"expansion factor of hyperscaler networks" default:"3"`
	CorporateDCExpansionFactor     float64 `help:"expansion factor of corporate data center networks" default:"4"`
	StorjExpansionFactor           float64 `help:"expansion factor of storj network" default:"2.7"`
	HyperscalerRegionCount         float64 `help:"region count of hyperscaler networks" default:"2"`
	CorporateDCRegionCount         float64 `help:"region count of corporate data center networks" default:"2"`
	StorjRegionCount               float64 `help:"region count of storj network" default:"1"`
	StorjStandardNetworkWeighting  float64 `help:"network weighting of already provisioned, powered drives, in fraction" default:"0.21"`
	StorjNewNetworkWeighting       float64 `help:"network weighting of new nodes, in fraction" default:"0.582"`
	HyperscalerUtilizationFraction float64 `help:"utilization fraction of hyperscaler networks, in fraction" default:"0.75"`
	CorporateDCUtilizationFraction float64 `help:"utilization fraction of corporate data center networks, in fraction" default:"0.40"`
	StorjUtilizationFraction       float64 `help:"utilization fraction of storj network, in fraction" default:"0.85"`
	AverageCO2SequesteredByTree    float64 `help:"weighted average CO2 sequestered by a medium growth coniferous or deciduous tree, in kgCO2e/tree" default:"60"`
}
