// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    HeldInfo,
    PaymentInfoParameters,
    PayoutApi,
    TotalPayoutInfo,
} from '@/app/types/payout';

/**
 * Mock for PayoutApi.
 */
export class PayoutApiMock implements PayoutApi {
    public getHeldInfoByMonth(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo> {
        return Promise.resolve(new HeldInfo());
    }

    public getHeldInfoByPeriod(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo> {
        return Promise.resolve(new HeldInfo());
    }

    public getTotal(paymentInfoParameters: PaymentInfoParameters): Promise<TotalPayoutInfo> {
        return Promise.resolve(new TotalPayoutInfo());
    }
}
