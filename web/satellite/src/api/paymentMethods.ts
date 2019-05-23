// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

// fetchProjectInvoices retrieves project invoices
export async function fetchProjectPaymentMethods(projectID: string): Promise<RequestResponse<PaymentMethod[]>> {
    let result: RequestResponse<PaymentMethod[]> = {
        errorMessage: '',
        isSuccess: false,
        data: [] as PaymentMethod[]
    };

    let response: any = await apollo.query(
        {
            query: gql(`
                query {
                    project(id: "${projectID}") {
                        paymentMethods {
                            expYear,
                            expMonth,
                            brand,
                            lastFour,
                            holderName,
                            addedAt,
                        }
                    }
                }`
            ),
            fetchPolicy: 'no-cache',
            errorPolicy: 'all'
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.project.paymentMethods;
    }

    return result;
}
