// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// PaymentMethod holds card information to display
declare type PaymentMethod = {
    id: string,
    expYear: number,
    expMonth: number,
    brand: string,
    lastFour: string,
    holderName: string,
    addedAt: Date,
    isDefault: boolean,
};

declare type AddPaymentMethodInput = {
    token: string,
    makeDefault: boolean,
};
