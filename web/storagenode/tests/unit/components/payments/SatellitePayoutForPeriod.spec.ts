// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.


import {SatellitePayoutForPeriod} from '@/storagenode/payouts/payouts';


describe('SatellitePayoutForPeriod', (): void => {
    it('ETH transactionLink', (): void => {
        const s = new SatellitePayoutForPeriod()
        s.receipt = "eth:0xCAFE"

        expect(s.transactionLink).toMatch("https://etherscan.io/tx/0xCAFE")
    });

    it('ZkSync transactionLink', (): void => {
        const s = new SatellitePayoutForPeriod()
        s.receipt = "zksync:0xCAFE"

        expect(s.transactionLink).toMatch("https://zkscan.io/explorer/transactions/0xCAFE")
    });

    it('ZkSync transactionLink without 0x', (): void => {
        const s = new SatellitePayoutForPeriod()
        s.receipt = "zksync:CAFE"

        expect(s.transactionLink).toMatch("https://zkscan.io/explorer/transactions/0xCAFE")
    });

    it('ZkSync transactionLink to L1', (): void => {
        const s = new SatellitePayoutForPeriod()
        s.receipt = "zkwithdraw:0xCAFE"

        expect(s.transactionLink).toMatch("https://zkscan.io/explorer/transactions/0xCAFE")
    });

    it('polygon transactionLink', (): void => {
        const s = new SatellitePayoutForPeriod()
        s.receipt = "polygon:0xCAFE"

        expect(s.transactionLink).toMatch("https://polygonscan.com/tx/0xCAFE")
    });

});
