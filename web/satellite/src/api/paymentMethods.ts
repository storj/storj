// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import gql from 'graphql-tag';

import { RequestResponse } from '@/types/response';
import apollo from '@/utils/apollo';

export async function addProjectPaymentMethodRequest(projectId: string, cardToken: string, isDefault: boolean): Promise<RequestResponse<null>> {
    const result: RequestResponse<null> = new RequestResponse<null>();

    const response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($projectId: String!, cardToken: String!, isDefault: Boolean!) {
                        addPaymentMethod(
                            projectID: $projectId,
                            cardToken: $cardToken,
                            isDefault: $isDefault
                        )
                }
            `),
            variables: {
                projectId: projectId,
                cardToken: cardToken,
                isDefault: isDefault,
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        },
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

export async function setDefaultPaymentMethodRequest(projectId: string, paymentId: string): Promise<RequestResponse<null>> {
   const result: RequestResponse<null> = new RequestResponse<null>();

   const response: any = await apollo.mutate(
       {
           mutation: gql(`
                mutation($projectId: String!, paymentId: String!) {
                    setDefaultPaymentMethod(
                        projectID: $projectId,
                        id: $paymentId
                    )
                }
           `),
           variables: {
               projectId: projectId,
               id: paymentId,
           },
           fetchPolicy: 'no-cache',
           errorPolicy: 'all',
       },
   );

   if (response.errors) {
       result.errorMessage = response.errors[0].message;
   } else {
       result.isSuccess = true;
   }

   return result;
}

export async function deletePaymentMethodRequest(paymentId: string): Promise<RequestResponse<null>> {
    const result: RequestResponse<null> = new RequestResponse<null>();

    const response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($id: String!) {
                    deletePaymentMethod(
                        id: $paymentId
                    )
                }
           `),
            variables: {
                id: paymentId,
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        },
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

// fetchProjectInvoices retrieves project invoices
export async function fetchProjectPaymentMethods(projectId: string): Promise<RequestResponse<PaymentMethod[]>> {
    const result: RequestResponse<PaymentMethod[]> = new RequestResponse<PaymentMethod[]>();

    const response: any = await apollo.query(
        {
            query: gql(`
                query($projectId: String!) {
                    project(id: $projectId) {
                        paymentMethods {
                            id,
                            expYear,
                            expMonth,
                            brand,
                            lastFour,
                            holderName,
                            addedAt,
                            isDefault
                        }
                    }
                }`,
            ),
            variables: {
                projectId: projectId,
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        },
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.project.paymentMethods;
    }

    return result;
}
