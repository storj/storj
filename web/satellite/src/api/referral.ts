// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { RequestResponse } from '@/types/response';

export async function fetchReferralInfo(): Promise<RequestResponse<ReferralInfo>> {
    let result: RequestResponse<ReferralInfo> = new RequestResponse<ReferralInfo>();

    let response: any = await apollo.query(
        {
            query: gql(`
                query {
                    activeReward (
                        type: 1,
                    ) {
                        awardCreditInCent,
                        redeemableCap,
                        awardCreditDurationDays,
                        inviteeCreditDurationDays,
                        redeemableCap,
                        expiresAt,
                        status,
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
        result.data = response.data.activeReward;
    }

    return result;
}
