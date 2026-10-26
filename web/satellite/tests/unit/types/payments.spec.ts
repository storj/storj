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
        expect(charge?.priceModel.minimumStorageBytes).toBe(0);
        expect(charge?.summary).toBe(1500);
    });

    it('reads the minimum storage of the product', (): void => {
        const charges = ProductCharges.fromJSON({
            charges: { 'project-a': { 1: { ...chargeJSON('0', 0), minimumStorageBytes: 1.2e15 } } },
            applyMinimumCharge: false,
        } as never);

        expect(charges.get('project-a', 1)?.priceModel.minimumStorageBytes).toBe(1.2e15);
    });

    it('adds the minimum storage charges to the price', (): void => {
        const charges = ProductCharges.fromJSON({
            charges: {
                'project-a': { 1: chargeJSON('0', 0) },
            },
            minimumStorageCharges: {
                2: {
                    productID: 2,
                    productName: 'advanced tier',
                    minimumStorageBytes: 5e12,
                    discountPercent: '0',
                    storage: 1000 * 1e9 * 720,
                    storagePrice: 700,
                    discountPrice: 0,
                },
                1: {
                    productID: 1,
                    productName: 'standard tier',
                    minimumStorageBytes: 1.2e15,
                    discountPercent: '25',
                    storage: 200000 * 1e9 * 720,
                    storagePrice: 140000,
                    discountPrice: 35000,
                },
            },
            applyMinimumCharge: false,
        } as never);

        // the minimum storage charges are sorted by product.
        expect(charges.minimumStorageCharges.map(charge => charge.productID)).toEqual([1, 2]);
        const minimumCharge = charges.minimumStorageCharges[0];
        expect(minimumCharge.productName).toBe('standard tier');
        expect(minimumCharge.minimumStorageBytes).toBe(1.2e15);
        expect(minimumCharge.discountPercent).toBe('25');
        expect(minimumCharge.summary).toBe(140000 - 35000);

        // they apply to the storage of all projects, so they're not part of any project's price.
        expect(charges.getProjectPrice('project-a')).toBe(1500);
        expect(charges.getPrice()).toBe(1500 + 105000 + 700);
    });

    it('adds the extra price of overage storage to the price', (): void => {
        const charges = ProductCharges.fromJSON({
            charges: {
                'project-a': { 1: chargeJSON('0', 0) },
            },
            minimumStorageCharges: {
                1: {
                    productID: 1,
                    productName: 'standard tier',
                    minimumStorageBytes: 1.2e15,
                    discountPercent: '25',
                    storage: 0,
                    storagePrice: 0,
                    overageStorage: 300000 * 1e9 * 720,
                    overageExtraCents: '0.0001',
                    overagePrice: 30000,
                    discountPrice: 7500,
                },
            },
            applyMinimumCharge: false,
        } as never);

        const minimumCharge = charges.minimumStorageCharges[0];
        expect(minimumCharge.overageStorage).toBe(300000 * 1e9 * 720);
        expect(minimumCharge.overageExtraCents).toBe('0.0001');
        expect(minimumCharge.summary).toBe(30000 - 7500);

        expect(charges.getProjectPrice('project-a')).toBe(1500);
        expect(charges.getPrice()).toBe(1500 + 22500);
    });

    it('reads minimum storage charges without overage', (): void => {
        const charges = ProductCharges.fromJSON({
            charges: {},
            minimumStorageCharges: {
                1: {
                    productID: 1,
                    productName: 'standard tier',
                    minimumStorageBytes: 1.2e15,
                    discountPercent: '0',
                    storage: 200000 * 1e9 * 720,
                    storagePrice: 140000,
                    discountPrice: 0,
                },
            },
            applyMinimumCharge: false,
        } as never);

        const minimumCharge = charges.minimumStorageCharges[0];
        expect(minimumCharge.overageStorage).toBe(0);
        expect(minimumCharge.overageExtraCents).toBe('0');
        expect(minimumCharge.overagePrice).toBe(0);
        expect(charges.getPrice()).toBe(140000);
    });

    it('reads charges without minimum storage charges', (): void => {
        for (const minimumStorageCharges of [undefined, null]) {
            const charges = ProductCharges.fromJSON({
                charges: { 'project-a': { 1: chargeJSON('0', 0) } },
                minimumStorageCharges,
                applyMinimumCharge: false,
            } as never);

            expect(charges.minimumStorageCharges).toEqual([]);
            expect(charges.getPrice()).toBe(1500);
        }
    });
});
