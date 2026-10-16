// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { describe, it, expect } from 'vitest';

import { ProductCharges } from '@/types/payments';

// chargeJSON returns the JSON of a product charge as the API returns it.
function chargeJSON(discountPercent: string, discountPrice: number) {
    return {
        productID: '1',
        productName: 'standard tier',
        since: '2026-09-01T00:00:00Z',
        before: '2026-10-01T00:00:00Z',
        storageMBMonthCents: '0.0007',
        egressMBCents: '0.0007',
        segmentMonthCents: '0',
        smallObjectFeeCents: '0.0007',
        minimumRetentionFeeCents: '0',
        egressOverageMode: false,
        egressDiscountRatio: 0,
        storageRemainderBytes: 0,
        minimumRetentionDuration: 0,
        storage: 0,
        egress: 0,
        segmentCount: 0,
        includedEgress: 0,
        remainderStorage: 0,
        retentionRemainder: 0,
        storagePrice: 1260,
        egressPrice: 70,
        segmentPrice: 0,
        smallObjectFeePrice: 140,
        minimumRetentionFeePrice: 30,
        discountPercent,
        discountPrice,
    };
}

describe('ProductCharges', (): void => {
    it('takes the discount off the prices', (): void => {
        const charges = ProductCharges.fromJSON({
            charges: {
                'project-a': { 1: chargeJSON('25', 375) },
                'project-b': { 1: chargeJSON('0', 0) },
            },
            applyMinimumCharge: false,
        } as never);

        // the discount covers every fee, the minimum retention fee included.
        const charge = charges.get('project-a', 1);
        expect(charge?.discountPrice).toBe(375);
        expect(charge?.priceModel.discountPercent).toBe('25');
        expect(charge?.summary).toBe(1260 + 70 + 140 + 30 - 375);

        expect(charges.getProjectPrice('project-a')).toBe(1125);
        expect(charges.getProjectPrice('project-b')).toBe(1500);
        expect(charges.getPrice()).toBe(1125 + 1500);
    });

    it('reads charges without a discount', (): void => {
        const json = chargeJSON('0', 0) as Partial<ReturnType<typeof chargeJSON>>;
        delete json.discountPercent;
        delete json.discountPrice;

        const charges = ProductCharges.fromJSON({
            charges: { 'project-a': { 1: json } },
            applyMinimumCharge: false,
        } as never);

        const charge = charges.get('project-a', 1);
        expect(charge?.discountPrice).toBe(0);
        expect(charge?.priceModel.discountPercent).toBe('0');
        expect(charge?.summary).toBe(1500);
    });
});
