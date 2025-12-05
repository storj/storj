// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { APIClient } from '@/api/index';
import { Operator } from '@/operators';
import { Cursor, Page } from '@/private/pagination';

/**
 * client for nodes controller of MND api.
 */
export class Operators extends APIClient {
    private readonly ROOT_PATH: string = '/api/v0/operators';

    /**
     * returns {@link Page} page of operators.
     *
     * @throws {@link BadRequestError}
     * This exception is thrown if the input is not a valid.
     *
     * @throws {@link UnauthorizedError}
     * Thrown if the auth cookie is missing or invalid.
     *
     * @throws {@link InternalError}
     * Thrown if something goes wrong on server side.
     */
    public async listPaginated(cursor: Cursor): Promise<Page<Operator>> {
        const path = `${this.ROOT_PATH}?limit=${cursor.limit}&page=${cursor.page}`;

        const response = await this.http.get(path);

        if (!response.ok) {
            await this.handleError(response);
        }

        const operatorsPageJson = await response.json();

        let operators: Operator[] = [];

        if (operatorsPageJson.operators) {
            operators = operatorsPageJson.operators.map(
                operator => new Operator(
                    operator.nodeId,
                    operator.email,
                    operator.wallet,
                    operator.walletFeatures,
                    operator.undistributed,
                ),
            );
        }

        return new Page<Operator>(
            operators,
            operatorsPageJson.offset,
            operatorsPageJson.limit,
            operatorsPageJson.currentPage,
            operatorsPageJson.pageCount,
            operatorsPageJson.totalCount,
        );
    }
}
