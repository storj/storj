// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { RequestResponse } from '@/types/response';

// fetchProjectUsage retrieves total project usage for a given period
export async function fetchProjectUsage(projectId: string, since: Date, before: Date): Promise<RequestResponse<ProjectUsage>> {
    let result: RequestResponse<ProjectUsage> = new RequestResponse<ProjectUsage>();

    let response: any = await apollo.query(
        {
            query: gql(`
                query($projectId: String!, $since: DateTime!, $before: DateTime!) {
                    project(id: $projectId) {
                        usage(since: $since, before: $before) {
                            storage,
                            egress,
                            objectCount,
                            since,
                            before
                        }
                    }
                }`
            ),
            variables: {
                projectId: projectId,
                since: since.toISOString(),
                before: before.toISOString()
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all'
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.project.usage;
    }

    return result;
}

export async function fetchCreditUsage(): Promise<RequestResponse<CreditUsage>> {
    let result: RequestResponse<CreditUsage> = new RequestResponse<CreditUsage>();

    let response: any = await apollo.query(
        {
            query: gql(`
                query {
                    creditUsage {
                        referred,
                        usedCredit,
                        availableCredit,
                    }
                }`
            ),
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.creditUsage;
    }

    return result;
}
