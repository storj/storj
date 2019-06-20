// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '@/utils/apolloManager';
import gql from 'graphql-tag';

export async function test(nodeId: string): Promise<number> {
    try {
        let response: any = await apolloManager.query(
            {
                query: gql(`
                    query {
                        
                    }`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            return 0;
        }

        return response.data.isNodeUp ? 1 : 0;

    } catch (e) {
        return 0;
    }
}
