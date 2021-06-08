// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { Operators as OperatorsClient } from '@/api/operators';
import { Cursor, Page } from '@/private/pagination';

/**
 *Operator contains contains SNO payouts contact details.
 */
export class Operator {
    public constructor(
        public email: string,
        public wallet: string,
        public walletFeatures: string[],
    ) {}

    /**
     * indicates if wallet features are enabled.
     */
    public get areWalletFeaturesEnabled(): boolean {
        return this.walletFeatures.length !== 0;
    }

    /**
     * generates link on etherscan.
     */
    public get etherscanLink(): string {
        // TODO: place this to config.
        return `https://etherscan.io/address/${this.wallet}`;
    }

    /**
     * generates link for zkscan.
     */
    public get zkscanLink(): string {
        // TODO: place this to config.
        return `https://zkscan.io/explorer/accounts/${this.wallet}`;
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
