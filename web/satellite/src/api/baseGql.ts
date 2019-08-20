// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import gql from 'graphql-tag';
import apolloManager from '@/utils/apollo';

/**
 * BaseGql is a graphql utility which allows to perform queries and mutations
 */
export class BaseGql {
    /**
     * performs qraphql query
     *
     * @param query - qraphql query
     * @param variables - variables to bind in query. null by default.
     * @throws Error
     */
    protected async query(query: string, variables: any = null): Promise<any> {
        let response: any = await apolloManager.query(
            {
                query: gql(query),
                variables,
                fetchPolicy: 'no-cache',
                errorPolicy: 'all',
            }
        );

        if (response.errors) {
            throw new Error(this.combineErrors(response.errors));
        }

        return response;
    }

    /**
     * performs qraphql mutation
     *
     * @param query - qraphql query
     * @param variables - variables to bind in query. null by default.
     * @throws Error
     */
    protected async mutate(query: string, variables: any = null): Promise<any> {
        let response: any = await apolloManager.mutate(
            {
                mutation: gql(query),
                variables,
                fetchPolicy: 'no-cache',
                errorPolicy: 'all',
            }
        );

        if (response.errors) {
            throw new Error(this.combineErrors(response.errors));
        }

        return response;
    }

    private combineErrors(gqlError: any): string {
        return gqlError.map(err => err.message).join('\n');
    }
}
