// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

import (
	"time"

	"github.com/zeebo/errs"
)

// Error describes internal emission service error.
var Error = errs.Class("emission service")

const (
	decimalMultiplier = 1000
	dayHours          = 24
	yearDays          = 365.25
	byteLabel         = "B"
	wattLabel         = "W"
	hourLabel         = "H"
	kilogramLabel     = "kg"
)

const (
	hyperscaler   = 0
	corporateDC   = 1
	storjStandard = 2
	storjReused   = 3
	storjNew      = 4
	modalityCount = 5
)

var (
	// B is a Val constructor function with byte (B) dimension.
	B = ValMaker(byteLabel)
	// KB is a Val constructor function with kilobyte (KB) dimension.
	KB = B(decimalMultiplier).Maker()
	// MB is a Val constructor function with megabyte (MB) dimension.
	MB = KB(decimalMultiplier).Maker()
	// GB is a Val constructor function with gigabyte (GB) dimension.
	GB = MB(decimalMultiplier).Maker()
	// TB is a Val constructor function with terabyte (TB) dimension.
	TB = GB(decimalMultiplier).Maker()

	// W is a Val constructor function with watt (W) dimension.
	W = ValMaker(wattLabel)
	// kW is a Val constructor function with kilowatt (kW) dimension.
	kW = W(decimalMultiplier).Maker()

	// H is a Val constructor function with hour (H) dimension.
	H = ValMaker(hourLabel)
	// Y is a Val constructor function with year (Y) dimension.
	Y = H(dayHours * yearDays).Maker()

	// kg is a Val constructor function with kilogram (kg) dimension.
	kg = ValMaker(kilogramLabel)
)

// Service is an emission service.
// Performs emissions impact calculations.
//
// architecture: Service
type Service struct {
	config Config
}

// NewService creates a new Service with the given configuration.
func NewService(config Config) *Service {
	return &Service{config: config}
}

// Impact represents emission impact from different sources.
type Impact struct {
	EstimatedKgCO2eStorj                       float64
	EstimatedKgCO2eHyperscaler                 float64
	EstimatedKgCO2eCorporateDC                 float64
	EstimatedFractionSavingsAgainstHyperscaler float64
	EstimatedFractionSavingsAgainstCorporateDC float64
}

// Row holds data row of predefined number of values.
type Row [modalityCount]*Val

// CalculateImpact calculates emission impact coming from different sources e.g. Storj, hyperscaler or corporateDC.
func (sv *Service) CalculateImpact(amountOfDataInTB float64, duration time.Duration) (*Impact, error) {
	// Define a data row of services expansion factors.
	expansionFactor := sv.prepareExpansionFactorRow()

	// Define a data row of services region count.
	regionCount := sv.prepareRegionCountRow()

	// Define a data row of services network weighting.
	networkWeighting, err := sv.prepareNetworkWeightingRow()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Define a data row of services utilization fractions.
	modalityUtilization := sv.prepareUtilizationRow()

	// Define a data row of services hard drive life period.
	driveLifetime := sv.prepareDriveLifetimeRow()

	// Define a data row of services hard drive embodied carbon emission.
	driveEmbodiedCarbon := sv.prepareDriveEmbodiedCarbonEmissionRow()

	// Define a data row of services hard drive amortized embodied carbon emission.
	amortizedEmbodiedCarbon := prepareDriveAmortizedEmbodiedCarbonEmissionRow(driveEmbodiedCarbon, driveLifetime)

	// Define a data row of services carbon emission from powering hard drives.
	carbonFromPower := sv.prepareCarbonFromDrivePoweringRow()

	timeStored := H(duration.Seconds() / (60 * 60))

	// Define a data row of services carbon emission from write and repair actions.
	carbonFromWritesAndRepairs, err := sv.prepareCarbonFromWritesAndRepairsRow(timeStored)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Define a data row of services carbon emission from metadata overhead.
	carbonPerByteMetadataOverhead, err := sv.prepareCarbonPerByteMetadataOverheadRow()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Define a data row of services total carbon emission per byte of data.
	carbonTotalPerByte, err := sumRows(amortizedEmbodiedCarbon, carbonFromPower, carbonFromWritesAndRepairs, carbonPerByteMetadataOverhead)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Define a data row of services effective carbon emission per byte of data.
	effectiveCarbonPerByte := prepareEffectiveCarbonPerByteRow(carbonTotalPerByte, modalityUtilization)

	// Define a data row of services total carbon emission.
	totalCarbon := prepareTotalCarbonRow(amountOfDataInTB, effectiveCarbonPerByte, expansionFactor, regionCount, timeStored)

	// Calculate Storj blended value.
	storjBlended, err := calculateStorjBlended(networkWeighting, totalCarbon)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Calculate emission impact per service.
	estimatedKgCO2eStorj, err := storjBlended.InUnits(kg(1))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	estimatedKgCO2eHyperscaler, err := totalCarbon[hyperscaler].InUnits(kg(1))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	estimatedKgCO2eCorporateDC, err := totalCarbon[corporateDC].InUnits(kg(1))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	rv := &Impact{
		EstimatedKgCO2eStorj:       estimatedKgCO2eStorj,
		EstimatedKgCO2eHyperscaler: estimatedKgCO2eHyperscaler,
		EstimatedKgCO2eCorporateDC: estimatedKgCO2eCorporateDC,
	}

	if rv.EstimatedKgCO2eHyperscaler != 0 {
		rv.EstimatedFractionSavingsAgainstHyperscaler = 1 - (rv.EstimatedKgCO2eStorj / rv.EstimatedKgCO2eHyperscaler)
	}

	if rv.EstimatedKgCO2eCorporateDC != 0 {
		rv.EstimatedFractionSavingsAgainstCorporateDC = 1 - (rv.EstimatedKgCO2eStorj / rv.EstimatedKgCO2eCorporateDC)
	}

	return rv, nil
}

func (sv *Service) prepareExpansionFactorRow() *Row {
	storjExpansionFactor := Q(sv.config.StorjExpansionFactor)

	row := new(Row)
	row[hyperscaler] = Q(sv.config.HyperscalerExpansionFactor)
	row[corporateDC] = Q(sv.config.CorporateDCExpansionFactor)
	row[storjStandard] = storjExpansionFactor
	row[storjReused] = storjExpansionFactor
	row[storjNew] = storjExpansionFactor

	return row
}

func (sv *Service) prepareRegionCountRow() *Row {
	storjRegionCount := Q(sv.config.StorjRegionCount)

	row := new(Row)
	row[hyperscaler] = Q(sv.config.HyperscalerRegionCount)
	row[corporateDC] = Q(sv.config.CorporateDCRegionCount)
	row[storjStandard] = storjRegionCount
	row[storjReused] = storjRegionCount
	row[storjNew] = storjRegionCount

	return row
}

func (sv *Service) prepareNetworkWeightingRow() (*Row, error) {
	row := new(Row)
	row[storjStandard] = Q(sv.config.StorjStandardNetworkWeighting)
	row[storjNew] = Q(sv.config.StorjNewNetworkWeighting)

	storjNotNewNodesFraction, err := Q(1).Sub(row[storjNew])
	if err != nil {
		return nil, err
	}

	storjReusedVal, err := storjNotNewNodesFraction.Sub(row[storjStandard])
	if err != nil {
		return nil, err
	}

	row[storjReused] = storjReusedVal

	return row, nil
}

func (sv *Service) prepareUtilizationRow() *Row {
	storjUtilizationFraction := Q(sv.config.StorjUtilizationFraction)

	row := new(Row)
	row[hyperscaler] = Q(sv.config.HyperscalerUtilizationFraction)
	row[corporateDC] = Q(sv.config.CorporateDCUtilizationFraction)
	row[storjStandard] = storjUtilizationFraction
	row[storjReused] = storjUtilizationFraction
	row[storjNew] = storjUtilizationFraction

	return row
}

func (sv *Service) prepareDriveLifetimeRow() *Row {
	standardDriveLife := Y(sv.config.StandardDriveLife)
	shortenedDriveLife := Y(sv.config.ShortenedDriveLife)
	extendedDriveLife := Y(sv.config.ExtendedDriveLife)

	row := new(Row)
	row[hyperscaler] = standardDriveLife
	row[corporateDC] = standardDriveLife
	row[storjStandard] = extendedDriveLife
	row[storjReused] = shortenedDriveLife
	row[storjNew] = extendedDriveLife

	return row
}

func (sv *Service) prepareDriveEmbodiedCarbonEmissionRow() *Row {
	newDriveEmbodiedCarbon := kg(sv.config.NewDriveEmbodiedCarbon).Div(TB(1))
	noEmbodiedCarbon := kg(0).Div(TB(1))

	row := new(Row)
	row[hyperscaler] = newDriveEmbodiedCarbon
	row[corporateDC] = newDriveEmbodiedCarbon
	row[storjStandard] = noEmbodiedCarbon
	row[storjReused] = noEmbodiedCarbon
	row[storjNew] = newDriveEmbodiedCarbon

	return row
}

func prepareDriveAmortizedEmbodiedCarbonEmissionRow(driveCarbonRow, driveLifetimeRow *Row) *Row {
	row := new(Row)
	for modality := 0; modality < modalityCount; modality++ {
		row[modality] = driveCarbonRow[modality].Div(driveLifetimeRow[modality])
	}

	return row
}

func (sv *Service) prepareCarbonFromDrivePoweringRow() *Row {
	carbonFromDrivePowering := kg(sv.config.CarbonFromDrivePowering).Div(TB(1).Mul(Y(1)))
	noCarbonFromDrivePowering := kg(0).Div(B(1).Mul(H(1)))

	row := new(Row)
	row[hyperscaler] = carbonFromDrivePowering
	row[corporateDC] = carbonFromDrivePowering
	row[storjStandard] = noCarbonFromDrivePowering
	row[storjReused] = carbonFromDrivePowering
	row[storjNew] = carbonFromDrivePowering

	return row
}

func (sv *Service) prepareCarbonFromWritesAndRepairsRow(timeStored *Val) (*Row, error) {
	writeEnergy := Q(sv.config.WriteEnergy).Mul(W(1)).Mul(H(1)).Div(GB(1))
	CO2PerEnergy := kg(sv.config.CO2PerEnergy).Div(kW(1).Mul(H(1)))
	noCarbonFromWritesAndRepairs := kg(0).Div(B(1).Mul(H(1)))

	// this raises 1+monthlyFractionOfDataRepaired to power of 12
	// TODO(jt): should we be doing this?
	dataRepaired := TB(sv.config.RepairedData)
	expandedData := TB(sv.config.ExpandedData)
	monthlyFractionOfDataRepaired := dataRepaired.Div(expandedData)
	repairFactor := Q(1)
	for i := 0; i < 12; i++ {
		monthlyFraction, err := Q(1).Add(monthlyFractionOfDataRepaired)
		if err != nil {
			return nil, err
		}

		repairFactor = repairFactor.Mul(monthlyFraction)
	}

	row := new(Row)
	row[hyperscaler] = noCarbonFromWritesAndRepairs
	row[corporateDC] = noCarbonFromWritesAndRepairs
	for modality := storjStandard; modality < modalityCount; modality++ {
		row[modality] = writeEnergy.Mul(CO2PerEnergy).Mul(repairFactor).Div(timeStored)
	}

	return row, nil
}

func (sv *Service) prepareCarbonPerByteMetadataOverheadRow() (*Row, error) {
	noCarbonPerByteMetadataOverhead := kg(0).Div(B(1).Mul(H(1)))

	row := new(Row)
	row[hyperscaler] = noCarbonPerByteMetadataOverhead
	row[corporateDC] = noCarbonPerByteMetadataOverhead

	storjGCPCarbon := kg(sv.config.StorjGCPCarbon)
	storjCRDBCarbon := kg(sv.config.StorjCRDBCarbon)

	monthlyStorjGCPAndCRDBCarbon, err := storjGCPCarbon.Add(storjCRDBCarbon)
	if err != nil {
		return nil, err
	}

	storjEdgeCarbon := kg(sv.config.StorjEdgeCarbon)

	monthlyStorjGCPAndCRDBAndEdgeCarbon, err := monthlyStorjGCPAndCRDBCarbon.Add(storjEdgeCarbon)
	if err != nil {
		return nil, err
	}

	storjAnnualCarbon := monthlyStorjGCPAndCRDBAndEdgeCarbon.Mul(Q(12))
	storjExpandedNetworkStorage := TB(sv.config.StorjExpandedNetworkStorage)
	carbonOverheadPerByte := storjAnnualCarbon.Div(storjExpandedNetworkStorage.Mul(Y(1)))

	for modality := storjStandard; modality < modalityCount; modality++ {
		row[modality] = carbonOverheadPerByte
	}

	return row, nil
}

func prepareEffectiveCarbonPerByteRow(carbonTotalOerByteRow, utilizationRow *Row) *Row {
	row := new(Row)
	for modality := 0; modality < modalityCount; modality++ {
		row[modality] = carbonTotalOerByteRow[modality].Div(utilizationRow[modality])
	}

	return row
}

func prepareTotalCarbonRow(amountOfDataInTB float64, effectiveCarbonPerByteRow, expansionFactorRow, regionCountRow *Row, timeStored *Val) *Row {
	amountOfData := TB(amountOfDataInTB)

	row := new(Row)
	for modality := 0; modality < modalityCount; modality++ {
		row[modality] = effectiveCarbonPerByteRow[modality].Mul(amountOfData).Mul(timeStored).Mul(expansionFactorRow[modality]).Mul(regionCountRow[modality])
	}

	return row
}

func calculateStorjBlended(networkWeightingRow, totalCarbonRow *Row) (*Val, error) {
	storjReusedTotalCarbon := networkWeightingRow[storjReused].Mul(totalCarbonRow[storjReused])
	storjNewAndReusedTotalCarbon, err := networkWeightingRow[storjNew].Mul(totalCarbonRow[storjNew]).Add(storjReusedTotalCarbon)
	if err != nil {
		return nil, err
	}

	storjBlended, err := networkWeightingRow[storjStandard].Mul(totalCarbonRow[storjStandard]).Add(storjNewAndReusedTotalCarbon)
	if err != nil {
		return nil, err
	}

	return storjBlended, nil
}

func sumRows(v ...*Row) (*Row, error) {
	rv := v[0]
	for _, l := range v[1:] {
		for i := 0; i < len(l); i++ {
			newVal, err := rv[i].Add(l[i])
			if err != nil {
				return nil, err
			}

			rv[i] = newVal
		}
	}
	return rv, nil
}
