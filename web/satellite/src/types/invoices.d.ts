// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// PaymentMethod holds card information to display
declare type PaymentMethod = {
    expYear: number,
    expMonth: number,
    brand: string,
    lastFour: string,
    holderName: string,
    addedAt: Date,
}

// ProjectInvoice holds information about project invoice
declare type ProjectInvoice = {
    projectID: string,
    number: string,
    status: string,
    amount: number,
    paymentMethod: PaymentMethod,
    startDate: Date,
    endDate: Date,
    downloadLink: string,
    createdAt: Date,
};
