// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { gql } from 'graphql-tag';
import { GraphQLError } from 'graphql';

import { apollo } from '@/utils/apollo';

/**
 * BaseGql is a graphql utility which allows to perform queries and mutations.
 */
export class BaseGql {
    /**
     * performs qraphql query.
     *
     * @param query - qraphql query
     * @param variables - variables to bind in query. null by default.
     * @throws Error
     */
    protected async query(query: string, variables: any = null): Promise<any> { // eslint-disable-line @typescript-eslint/no-explicit-any
        const response = await apollo.query(
            {
                query: gql(query),
                variables,
                fetchPolicy: 'no-cache',
                errorPolicy: 'all',
            },
        );

        if (response.errors) {
            throw new Error(this.combineErrors(response.errors));
        }

        return response;
    }

    /**
     * performs qraphql mutation.
     *
     * @param query - qraphql query
     * @param variables - variables to bind in query. null by default.
     * @throws Error
     */
    protected async mutate(query: string, variables: any = null): Promise<any> { // eslint-disable-line @typescript-eslint/no-explicit-any
        const response = await apollo.mutate(
            {
                mutation: gql(query),
                variables,
                fetchPolicy: 'no-cache',
                errorPolicy: 'all',
            },
        );

        if (response.errors) {
            throw new Error(this.combineErrors(response.errors));
        }

        return response;
    }

    private combineErrors(gqlError: readonly GraphQLError[]): string {
        return gqlError.map(err => err).join('\n');
    }
}
