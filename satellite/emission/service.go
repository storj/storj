// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

import (
	"math"
	"time"

	"github.com/zeebo/errs"
)

// Error describes internal emission service error.
var Error = errs.Class("emission service")

const (
	decimalMultiplier   = 1000
	tbToBytesMultiplier = 1e-12
	gbToBytesMultiplier = 1e-9
	dayHours            = 24
	yearDays            = 365.25
	twelveMonths        = 12
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
	unitless = Unit{}
	byteUnit = Unit{byte: 1}
	hour     = Unit{hour: 1}
	kilogram = Unit{kilogram: 1}

	wattHourPerByte     = Unit{watt: 1, hour: 1, byte: -1}
	kilogramPerWattHour = Unit{kilogram: 1, watt: -1, hour: -1}
	kilogramPerByte     = Unit{kilogram: 1, byte: -1}
	kilogramPerByteHour = Unit{kilogram: 1, byte: -1, hour: -1}
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
type Row [modalityCount]Val

// CalculationInput holds input data needed to perform emission impact calculations.
type CalculationInput struct {
	AmountOfDataInTB float64       // The amount of data in terabytes or terabyte-duration.
	Duration         time.Duration // The Duration over which the data is measured.
}

// CalculateImpact calculates emission impact coming from different sources e.g. Storj, hyperscaler or corporateDC.
func (sv *Service) CalculateImpact(input *CalculationInput) (*Impact, error) {
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

	timeStored := hour.Value(input.Duration.Seconds() / 3600)

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
	totalCarbon := prepareTotalCarbonRow(input, effectiveCarbonPerByte, expansionFactor, regionCount, timeStored)

	// Calculate Storj blended value.
	storjBlended, err := calculateStorjBlended(networkWeighting, totalCarbon)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// Calculate emission impact per service.
	oneKilogram := kilogram.Value(1)
	rv := &Impact{
		EstimatedKgCO2eStorj:       storjBlended.Div(oneKilogram).Value,
		EstimatedKgCO2eHyperscaler: totalCarbon[hyperscaler].Div(oneKilogram).Value,
		EstimatedKgCO2eCorporateDC: totalCarbon[corporateDC].Div(oneKilogram).Value,
	}

	if rv.EstimatedKgCO2eHyperscaler != 0 {
		estimatedFractionSavingsAgainstHyperscaler, err := unitless.Value(1).Sub(unitless.Value(rv.EstimatedKgCO2eStorj / rv.EstimatedKgCO2eHyperscaler))
		if err != nil {
			return nil, Error.Wrap(err)
		}

		rv.EstimatedFractionSavingsAgainstHyperscaler = estimatedFractionSavingsAgainstHyperscaler.Value
	}

	if rv.EstimatedKgCO2eCorporateDC != 0 {
		estimatedFractionSavingsAgainstCorporateDC, err := unitless.Value(1).Sub(unitless.Value(rv.EstimatedKgCO2eStorj / rv.EstimatedKgCO2eCorporateDC))
		if err != nil {
			return nil, Error.Wrap(err)
		}

		rv.EstimatedFractionSavingsAgainstCorporateDC = estimatedFractionSavingsAgainstCorporateDC.Value
	}

	return rv, nil
}

// CalculateSavedTrees calculates saved trees count based on emission impact.
func (sv *Service) CalculateSavedTrees(impact float64) int64 {
	return int64(math.Round(impact / sv.config.AverageCO2SequesteredByTree))
}

func (sv *Service) prepareExpansionFactorRow() *Row {
	storjExpansionFactor := unitless.Value(sv.config.StorjExpansionFactor)

	row := new(Row)
	row[hyperscaler] = unitless.Value(sv.config.HyperscalerExpansionFactor)
	row[corporateDC] = unitless.Value(sv.config.CorporateDCExpansionFactor)
	row[storjStandard] = storjExpansionFactor
	row[storjReused] = storjExpansionFactor
	row[storjNew] = storjExpansionFactor

	return row
}

func (sv *Service) prepareRegionCountRow() *Row {
	storjRegionCount := unitless.Value(sv.config.StorjRegionCount)

	row := new(Row)
	row[hyperscaler] = unitless.Value(sv.config.HyperscalerRegionCount)
	row[corporateDC] = unitless.Value(sv.config.CorporateDCRegionCount)
	row[storjStandard] = storjRegionCount
	row[storjReused] = storjRegionCount
	row[storjNew] = storjRegionCount

	return row
}

func (sv *Service) prepareNetworkWeightingRow() (*Row, error) {
	row := new(Row)
	row[storjStandard] = unitless.Value(sv.config.StorjStandardNetworkWeighting)
	row[storjNew] = unitless.Value(sv.config.StorjNewNetworkWeighting)
	storjNotNewNodesFraction, err := unitless.Value(1).Sub(row[storjNew])
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
	storjUtilizationFraction := unitless.Value(sv.config.StorjUtilizationFraction)

	row := new(Row)
	row[hyperscaler] = unitless.Value(sv.config.HyperscalerUtilizationFraction)
	row[corporateDC] = unitless.Value(sv.config.CorporateDCUtilizationFraction)
	row[storjStandard] = storjUtilizationFraction
	row[storjReused] = storjUtilizationFraction
	row[storjNew] = storjUtilizationFraction

	return row
}

func (sv *Service) prepareDriveLifetimeRow() *Row {
	standardDriveLife := yearsToHours(sv.config.StandardDriveLife)
	shortenedDriveLife := yearsToHours(sv.config.ShortenedDriveLife)
	extendedDriveLife := yearsToHours(sv.config.ExtendedDriveLife)

	row := new(Row)
	row[hyperscaler] = standardDriveLife
	row[corporateDC] = standardDriveLife
	row[storjStandard] = extendedDriveLife
	row[storjReused] = shortenedDriveLife
	row[storjNew] = extendedDriveLife

	return row
}

func (sv *Service) prepareDriveEmbodiedCarbonEmissionRow() *Row {
	newDriveEmbodiedCarbon := kilogramPerByte.Value(sv.config.NewDriveEmbodiedCarbon * tbToBytesMultiplier)
	noEmbodiedCarbon := kilogramPerByte.Value(0)

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
	carbonFromDrivePowering := kilogramPerByteHour.Value(sv.config.CarbonFromDrivePowering * tbToBytesMultiplier / dayHours / yearDays)
	noCarbonFromDrivePowering := kilogramPerByteHour.Value(0)

	row := new(Row)
	row[hyperscaler] = carbonFromDrivePowering
	row[corporateDC] = carbonFromDrivePowering
	row[storjStandard] = noCarbonFromDrivePowering
	row[storjReused] = carbonFromDrivePowering
	row[storjNew] = carbonFromDrivePowering

	return row
}

func (sv *Service) prepareCarbonFromWritesAndRepairsRow(timeStored Val) (*Row, error) {
	writeEnergy := wattHourPerByte.Value(sv.config.WriteEnergy * gbToBytesMultiplier)
	CO2PerEnergy := kilogramPerWattHour.Value(sv.config.CO2PerEnergy / decimalMultiplier)
	noCarbonFromWritesAndRepairs := kilogramPerByteHour.Value(0)

	// this raises 1+monthlyFractionOfDataRepaired to power of 12
	// TODO(jt): should we be doing this?
	dataRepaired := byteUnit.Value(sv.config.RepairedData / tbToBytesMultiplier)
	expandedData := byteUnit.Value(sv.config.ExpandedData / tbToBytesMultiplier)
	monthlyFractionOfDataRepaired := dataRepaired.Div(expandedData)
	repairFactor := unitless.Value(1)
	for i := 0; i < twelveMonths; i++ {
		monthlyFraction, err := unitless.Value(1).Add(monthlyFractionOfDataRepaired)
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
	noCarbonPerByteMetadataOverhead := kilogramPerByteHour.Value(0)

	row := new(Row)
	row[hyperscaler] = noCarbonPerByteMetadataOverhead
	row[corporateDC] = noCarbonPerByteMetadataOverhead

	storjGCPCarbon := kilogram.Value(sv.config.StorjGCPCarbon)
	storjCRDBCarbon := kilogram.Value(sv.config.StorjCRDBCarbon)

	monthlyStorjGCPAndCRDBCarbon, err := storjGCPCarbon.Add(storjCRDBCarbon)
	if err != nil {
		return nil, err
	}

	storjEdgeCarbon := kilogram.Value(sv.config.StorjEdgeCarbon)

	monthlyStorjGCPAndCRDBAndEdgeCarbon, err := monthlyStorjGCPAndCRDBCarbon.Add(storjEdgeCarbon)
	if err != nil {
		return nil, err
	}

	storjAnnualCarbon := monthlyStorjGCPAndCRDBAndEdgeCarbon.Mul(unitless.Value(twelveMonths))
	storjExpandedNetworkStorage := byteUnit.Value(sv.config.StorjExpandedNetworkStorage / tbToBytesMultiplier)
	carbonOverheadPerByte := storjAnnualCarbon.Div(storjExpandedNetworkStorage.Mul(yearsToHours(1)))

	for modality := storjStandard; modality < modalityCount; modality++ {
		row[modality] = carbonOverheadPerByte
	}

	return row, nil
}

func yearsToHours(v float64) Val {
	return hour.Value(v * yearDays * dayHours)
}

func prepareEffectiveCarbonPerByteRow(carbonTotalPerByteRow, utilizationRow *Row) *Row {
	row := new(Row)
	for modality := 0; modality < modalityCount; modality++ {
		row[modality] = carbonTotalPerByteRow[modality].Div(utilizationRow[modality])
	}

	return row
}

func prepareTotalCarbonRow(input *CalculationInput, effectiveCarbonPerByteRow, expansionFactorRow, regionCountRow *Row, timeStored Val) *Row {
	amountOfData := byteUnit.Value(input.AmountOfDataInTB / tbToBytesMultiplier)

	row := new(Row)
	for modality := 0; modality < modalityCount; modality++ {
		row[modality] = effectiveCarbonPerByteRow[modality].Mul(amountOfData).Mul(timeStored).Mul(expansionFactorRow[modality]).Mul(regionCountRow[modality])
	}

	return row
}

func calculateStorjBlended(networkWeightingRow, totalCarbonRow *Row) (Val, error) {
	storjReusedTotalCarbon := networkWeightingRow[storjReused].Mul(totalCarbonRow[storjReused])
	storjNewAndReusedTotalCarbon, err := networkWeightingRow[storjNew].Mul(totalCarbonRow[storjNew]).Add(storjReusedTotalCarbon)
	if err != nil {
		return Val{}, err
	}

	storjBlended, err := networkWeightingRow[storjStandard].Mul(totalCarbonRow[storjStandard]).Add(storjNewAndReusedTotalCarbon)
	if err != nil {
		return Val{}, err
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
