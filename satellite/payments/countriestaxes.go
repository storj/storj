// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import "github.com/stripe/stripe-go/v81"

// The countries and taxes lists are sourced from the Stripe Tax ID documentation
// at https://docs.stripe.com/billing/customer/tax-ids#supported-tax-id .

// CountryCode is the ISO 3166-1 alpha-2 country codes that Stripe supports for tax reporting.
type CountryCode string

// TaxCode the tax codes that Stripe supports for tax reporting.
type TaxCode string

const (
	ad CountryCode = "AD"
	ar CountryCode = "AR"
	au CountryCode = "AU"
	at CountryCode = "AT"
	be CountryCode = "BE"
	bo CountryCode = "BO"
	br CountryCode = "BR"
	bg CountryCode = "BG"
	ca CountryCode = "CA"
	cl CountryCode = "CL"
	cn CountryCode = "CN"
	co CountryCode = "CO"
	cr CountryCode = "CR"
	hr CountryCode = "HR"
	cy CountryCode = "CY"
	cz CountryCode = "CZ"
	dk CountryCode = "DK"
	do CountryCode = "DO"
	ec CountryCode = "EC"
	eg CountryCode = "EG"
	sv CountryCode = "SV"
	ee CountryCode = "EE"
	fi CountryCode = "FI"
	fr CountryCode = "FR"
	ge CountryCode = "GE"
	de CountryCode = "DE"
	gr CountryCode = "GR"
	hk CountryCode = "HK"
	hu CountryCode = "HU"
	is CountryCode = "IS"
	in CountryCode = "IN"
	id CountryCode = "ID"
	ie CountryCode = "IE"
	il CountryCode = "IL"
	it CountryCode = "IT"
	jp CountryCode = "JP"
	ke CountryCode = "KE"
	lv CountryCode = "LV"
	li CountryCode = "LI"
	lt CountryCode = "LT"
	lu CountryCode = "LU"
	my CountryCode = "MY"
	mt CountryCode = "MT"
	mx CountryCode = "MX"
	nl CountryCode = "NL"
	nz CountryCode = "NZ"
	no CountryCode = "NO"
	om CountryCode = "OM"
	pe CountryCode = "PE"
	ph CountryCode = "PH"
	pl CountryCode = "PL"
	pt CountryCode = "PT"
	ro CountryCode = "RO"
	ru CountryCode = "RU"
	sa CountryCode = "SA"
	rs CountryCode = "RS"
	sg CountryCode = "SG"
	sk CountryCode = "SK"
	si CountryCode = "SI"
	za CountryCode = "ZA"
	kr CountryCode = "KR"
	es CountryCode = "ES"
	se CountryCode = "SE"
	ch CountryCode = "CH"
	tw CountryCode = "TW"
	th CountryCode = "TH"
	tr CountryCode = "TR"
	ua CountryCode = "UA"
	ae CountryCode = "AE"
	gb CountryCode = "GB"
	us CountryCode = "US"
	uy CountryCode = "UY"
	ve CountryCode = "VE"
	vn CountryCode = "VN"
)

// TaxCountry is a country that Stripe supports for tax reporting.
type TaxCountry struct {
	Name string      `json:"name"`
	Code CountryCode `json:"code"`
}

// Tax is a tax that Stripe supports for tax reporting.
type Tax struct {
	Code        stripe.TaxIDType `json:"code"`
	Name        string           `json:"name"`
	Example     string           `json:"example"`
	CountryCode CountryCode      `json:"countryCode"`
}

// TaxCountries is a list of all countries whose taxes Stripe supports.
var TaxCountries = []TaxCountry{
	{"Andorra", ad},
	{"Argentina", ar},
	{"Australia", au},
	{"Austria", at},
	{"Belgium", be},
	{"Bolivia", bo},
	{"Brazil", br},
	{"Bulgaria", bg},
	{"Canada", ca},
	{"Chile", cl},
	{"China", cn},
	{"Colombia", co},
	{"Costa Rica", cr},
	{"Croatia", hr},
	{"Cyprus", cy},
	{"Czech Republic", cz},
	{"Denmark", dk},
	{"Dominican Republic", do},
	{"Ecuador", ec},
	{"Egypt", eg},
	{"El Salvador", sv},
	{"Estonia", ee},
	{"Finland", fi},
	{"France", fr},
	{"Georgia", ge},
	{"Germany", de},
	{"Greece", gr},
	{"Hong Kong", hk},
	{"Hungary", hu},
	{"Iceland", is},
	{"India", in},
	{"Indonesia", id},
	{"Ireland", ie},
	{"Israel", il},
	{"Italy", it},
	{"Japan", jp},
	{"Kenya", ke},
	{"Latvia", lv},
	{"Liechtenstein", li},
	{"Lithuania", lt},
	{"Luxembourg", lu},
	{"Malaysia", my},
	{"Malta", mt},
	{"Mexico", mx},
	{"Netherlands", nl},
	{"New Zealand", nz},
	{"Norway", no},
	{"Oman", om},
	{"Peru", pe},
	{"Philippines", ph},
	{"Poland", pl},
	{"Portugal", pt},
	{"Romania", ro},
	{"Russia", ru},
	{"Saudi Arabia", sa},
	{"Serbia", rs},
	{"Singapore", sg},
	{"Slovakia", sk},
	{"Slovenia", si},
	{"South Africa", za},
	{"South Korea", kr},
	{"Spain", es},
	{"Sweden", se},
	{"Switzerland", ch},
	{"Taiwan", tw},
	{"Thailand", th},
	{"Turkey", tr},
	{"Ukraine", ua},
	{"United Arab Emirates", ae},
	{"United Kingdom", gb},
	{"United States", us},
	{"Uruguay", uy},
	{"Venezuela", ve},
	{"Vietnam", vn},
}

// Taxes is a list of all taxes that Stripe supports.
var Taxes = []Tax{
	{stripe.TaxIDTypeADNRT, "Andorran NRT number", "A-123456-Z", ad},
	{stripe.TaxIDTypeARCUIT, "Argentinian tax ID number", "12-3456789-01", ar},
	{stripe.TaxIDTypeAUABN, "Australian Business Number (AU ABN)", "12345678912", au},
	{stripe.TaxIDTypeAUARN, "Australian Taxation Office Reference Number", "123456789123", au},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "ATU12345678", at},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "BE0123456789", be},
	{stripe.TaxIDTypeBOTIN, "Bolivian tax ID", "123456789", bo},
	{stripe.TaxIDTypeBRCNPJ, "Brazilian CNPJ number", "01.234.456/5432-10", br},
	{stripe.TaxIDTypeBRCPF, "Brazilian CPF number", "123.456.789-87", br},
	{stripe.TaxIDTypeBGUIC, "Bulgaria Unified Identification Code", "123456789", bg},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "BG0123456789", bg},
	{stripe.TaxIDTypeCABN, "Canadian BN", "123456789", ca},
	{stripe.TaxIDTypeCAGSTHST, "Canadian GST/HST number", "123456789RT0002", ca},
	{stripe.TaxIDTypeCAPSTBC, "Canadian PST number (British Columbia)", "PST-1234-5678", ca},
	{stripe.TaxIDTypeCAPSTMB, "Canadian PST number (Manitoba)", "123456-7", ca},
	{stripe.TaxIDTypeCAPSTSK, "Canadian PST number (Saskatchewan)", "1234567", ca},
	{stripe.TaxIDTypeCAQST, "Canadian QST number (Québec)", "1234567890TQ1234", ca},
	{stripe.TaxIDTypeCLTIN, "Chilean TIN", "12.345.678-K", cl},
	{stripe.TaxIDTypeCNTIN, "Chinese tax ID", "123456789012345000", cn},
	{stripe.TaxIDTypeCONIT, "Colombian NIT number", "123.456.789-0", co},
	{stripe.TaxIDTypeCRTIN, "Costa Rican tax ID", "1-234-567890", cr},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "HR12345678912", hr},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "CY12345678Z", cy},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "CZ1234567890", cz},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "DK12345678", dk},
	{stripe.TaxIDTypeDORCN, "Dominican RCN number", "123-4567890-1", do},
	{stripe.TaxIDTypeECRUC, "Ecuadorian RUC number", "1234567890001", ec},
	{stripe.TaxIDTypeEGTIN, "Egyptian Tax Identification Number", "123456789", eg},
	{stripe.TaxIDTypeSVNIT, "El Salvadorian NIT number", "1234-567890-123-4", sv},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "EE123456789", ee},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "FI12345678", fi},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "FRAB123456789", fr},
	{stripe.TaxIDTypeGEVAT, "Georgian VAT", "123456789", ge},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "DE123456789", de},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "EL123456789", gr},
	{stripe.TaxIDTypeHKBR, "Hong Kong BR number", "12345678", hk},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "HU12345678", hu},
	{stripe.TaxIDTypeHUTIN, "Hungary tax number (adószám)", "12345678-1-23", hu},
	{stripe.TaxIDTypeISVAT, "Icelandic VAT", "123456", is},
	{stripe.TaxIDTypeINGST, "Indian GST number", "12ABCDE3456FGZH", in},
	{stripe.TaxIDTypeIDNPWP, "Indonesian NPWP number", "12.345.678.9-012.345", id},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "IE1234567AB", ie},
	{stripe.TaxIDTypeILVAT, "Israel VAT", "12345", il},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "IT12345678912", it},
	{stripe.TaxIDTypeJPCN, "Japanese Corporate Number (*Hōjin Bangō*)", "1234567891234", jp},
	{stripe.TaxIDTypeJPRN, "Japanese Registered Foreign Businesses' Registration Number (*Tōroku Kokugai Jigyōsha no Tōroku Bangō*)", "12345", jp},
	{stripe.TaxIDTypeJPTRN, "Japanese Tax Registration Number (*Tōroku Bangō*)", "T1234567891234", jp},
	{stripe.TaxIDTypeKEPIN, "Kenya Revenue Authority Personal Identification Number", "P000111111A", ke},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "LV12345678912", lv},
	{stripe.TaxIDTypeLIUID, "Liechtensteinian UID number", "CHE123456789", li},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "LT123456789123", lt},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "LU12345678", lu},
	{stripe.TaxIDTypeMYFRP, "Malaysian FRP number", "12345678", my},
	{stripe.TaxIDTypeMYITN, "Malaysian ITN", "C 1234567890", my},
	{stripe.TaxIDTypeMYSST, "Malaysian SST number", "A12-3456-78912345", my},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "MT12345678", mt},
	{stripe.TaxIDTypeMXRFC, "Mexican RFC number", "ABC010203AB9", mx},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "NL123456789B12", nl},
	{stripe.TaxIDTypeNZGST, "New Zealand GST number", "123456789", nz},
	{stripe.TaxIDTypeNOVAT, "Norwegian VAT number", "123456789MVA", no},
	{stripe.TaxIDTypePERUC, "Peruvian RUC number", "12345678901", pe},
	{stripe.TaxIDTypePHTIN, "Philippines Tax Identification Number", "123456789012", ph},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "PL1234567890", pl},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "PT123456789", pt},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "RO1234567891", ro},
	{stripe.TaxIDTypeROTIN, "Romanian tax ID number", "1234567890123", ro},
	{stripe.TaxIDTypeRUINN, "Russian INN", "1234567891", ru},
	{stripe.TaxIDTypeRUKPP, "Russian KPP", "123456789", ru},
	{stripe.TaxIDTypeSAVAT, "Saudi Arabia VAT", "123456789012345", sa},
	{stripe.TaxIDTypeRSPIB, "Serbian PIB number", "123456789", rs},
	{stripe.TaxIDTypeSGGST, "Singaporean GST", "M12345678X", sg},
	{stripe.TaxIDTypeSGUEN, "Singaporean UEN", "123456789F", sg},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "SK1234567891", sk},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "SI12345678", si},
	{stripe.TaxIDTypeSITIN, "Slovenia tax number (davčna številka)", "12345678", si},
	{stripe.TaxIDTypeZAVAT, "South African VAT number", "4123456789", za},
	{stripe.TaxIDTypeKRBRN, "Korean BRN", "123-45-67890", kr},
	{stripe.TaxIDTypeESCIF, "Spanish NIF number (previously Spanish CIF number)", "A12345678", es},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "ESA1234567Z", es},
	{stripe.TaxIDTypeEUVAT, "European VAT number", "SE123456789123", se},
	{stripe.TaxIDTypeCHVAT, "Switzerland VAT number", "CHE-123.456.789 MWST", ch},
	{stripe.TaxIDTypeTWVAT, "Taiwanese VAT", "12345678", tw},
	{stripe.TaxIDTypeTHVAT, "Thai VAT", "1234567891234", th},
	{stripe.TaxIDTypeTRTIN, "Turkish Tax Identification Number", "123456789", tr},
	{stripe.TaxIDTypeUAVAT, "Ukrainian VAT", "123456789", ua},
	{stripe.TaxIDTypeAETRN, "United Arab Emirates TRN", "123456789012345", ae},
	{stripe.TaxIDTypeEUVAT, "Northern Ireland VAT number", "XI123456789", gb},
	{stripe.TaxIDTypeGBVAT, "United Kingdom VAT number", "GB123456789", gb},
	{stripe.TaxIDTypeUSEIN, "United States EIN", "12-3456789", us},
	{stripe.TaxIDTypeUYRUC, "Uruguayan RUC number", "123456789012", uy},
	{stripe.TaxIDTypeVERIF, "Venezuelan RIF number", "A-12345678-9", ve},
	{stripe.TaxIDTypeVNTIN, "Vietnamese tax ID number", "1234567890", vn},
}
