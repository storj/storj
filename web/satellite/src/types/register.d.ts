// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

declare type RegisterData = {
	firstName: string,
	firstNameError: string,
	lastName: string,
	lastNameError: string,
	email: string,
	emailError: string,
	password: string,
	passwordError: string,
	repeatedPassword: string,
	repeatedPasswordError: string,
	companyName: string,
	companyAddress: string,
	country: string,
	city: string,
	state: string,
	postalCode: string,
	isTermsAccepted: boolean,
	isTermsAcceptedError: boolean,
	optionalAreaShown: boolean
}