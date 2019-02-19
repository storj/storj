// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '@/utils/apolloManager';
import gql from 'graphql-tag';

export async function isNodeUp(nodeId: string): Promise<boolean> {
    try {
        let response: any = await apolloManager.query(
            {
                query: gql(`
                    query {
                        isNodeUp (
                            nodeID: "${nodeId}"
                        )
                    }`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            return false;
        }

        return response.data.status;

    } catch (e) {
        return false;
    }
}
