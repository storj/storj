// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

declare type User = {
    firstName: string,
    lastName: string,
    email: string,
    id: string,
    company: Company
}

declare type Company = {
    name: string,
    address: string,
    country: string,
    city: string,
    state: string,
    postalCode: string
}
