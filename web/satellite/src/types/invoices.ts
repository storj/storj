// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class PaymentMethod {
    public id: string;
    public expYear: number;
    public expMonth: number;
    public brand: string;
    public lastFour: string;
    public holderName: string;
    public addedAt: Date;
    public isDefault: boolean;

    public constructor(id: string = '',
                       expYear: number = 0,
                       expMonth: number = 0,
                       brand: string = '',
                       lastFour: string = '',
                       holderName: string = '',
                       addedAt: Date = new Date(),
                       isDefault: boolean = false) {
        this.id = id;
        this.expYear = expYear;
        this.expMonth = expMonth;
        this.brand = brand;
        this.lastFour = lastFour;
        this.holderName = holderName;
        this.addedAt = addedAt;

        this.isDefault = isDefault;
    }

    public addedAtFormatted(): string {
        if (!this.addedAt) {
            return '';
        }

        return new Date(this.addedAt).toLocaleDateString();
    }
}

export class AddPaymentMethodInput {
    public token: string;
    public makeDefault: boolean;

    public constructor(token: string, makeDefault: boolean) {
        this.token = token;

        this.makeDefault = makeDefault;
    }
}
