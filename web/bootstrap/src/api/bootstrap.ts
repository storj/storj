// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { NodeStatus } from '@/types/nodeStatus';

export async function checkAvailability(nodeId: string): Promise<number> {
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
            return NodeStatus.Error;
        }

        return response.data.isNodeUp ? NodeStatus.Active : NodeStatus.Error;

    } catch (e) {
        return NodeStatus.Error;
    }
}
