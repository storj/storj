// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { Operators as OperatorsClient } from '@/api/operators';
import { Cursor, Page } from '@/private/pagination';

/**
 * Divider to convert payout amounts to cents.
 */
const PRICE_DIVIDER = 10000;

/**
 *Operator contains contains SNO payouts contact details and amount of undistributed payouts.
 */
export class Operator {
    public constructor(
        public nodeId: string,
        public email: string,
        public wallet: string,
        public walletFeatures: string[] | null,
        public undistributed: number,
    ) {
        this.undistributed = this.convertToCents(this.undistributed);
    }

    /**
     * indicates if wallet features are enabled.
     */
    public get areWalletFeaturesEnabled(): boolean {
        return !!(this.walletFeatures && this.walletFeatures.length !== 0);
    }

    /**
     * generates link on etherscan.
     */
    public get etherscanLink(): string {
        // TODO: place this to config.
        return `https://etherscan.io/address/${this.wallet}#tokentxns`;
    }

    /**
     * generates link for zkscan.
     */
    public get zkscanLink(): string {
        // TODO: place this to config.
        return `https://explorer.zksync.io/address/${this.wallet}`;
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * exposes Operator related functionality.
 */
export class Operators {
    private operators: OperatorsClient;

    constructor(operators: OperatorsClient) {
        this.operators = operators;
    }

    public async listPaginated(cursor: Cursor): Promise<Page<Operator>> {
        return await this.operators.listPaginated(cursor);
    }
}
