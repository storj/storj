// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

// fetchProjectInvoices retrieves project invoices
export async function fetchProjectInvoices(projectID: string): Promise<RequestResponse<ProjectInvoice[]>> {
    let result: RequestResponse<ProjectInvoice[]> = {
        errorMessage: '',
        isSuccess: false,
        data: [] as ProjectInvoice[]
    };

    let response: any = await apollo.query(
        {
            query: gql(`
                query {
                    project(id: "${projectID}") {
                        invoices {
                            projectID,
                            invoiceID,
                            status,
                            amount,
                            paymentMethod{
                                brand,
                                lastFour,
                            }
                            startDate,
                            endDate,
                            downloadLink,
                            createdAt
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
        result.data = response.data.project.invoices;
    }

    return result;
}
