// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { RequestResponse } from '@/types/response';

export async function addProjectPaymentMethodRequest(projectId: string, cardToken: string, makeDefault: boolean): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($projectId: String!, cardToken: String!, isDefault: Boolean!) {
                        addPaymentMethod(
                            projectID: $projectId,
                            cardToken: $cardToken,
                            isDefault: $makeDefault
                        ) 
                }
            `),
            variables: {
                projectId: projectId,
                cardToken: cardToken,
                isDefault: makeDefault
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all'
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

export async function setDefaultPaymentMethodRequest(projectId: string, paymentId: string): Promise<RequestResponse<null>> {
   let result: RequestResponse<null> = new RequestResponse<null>();

   let response: any = await apollo.mutate(
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
               id: paymentId
           },
           fetchPolicy: 'no-cache',
           errorPolicy: 'all'
       }
   );

   if (response.errors) {
       result.errorMessage = response.errors[0].message;
   } else {
       result.isSuccess = true;
   }

   return result;
}

export async function deletePaymentMethodRequest(paymentId: string):Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($id: String!) {
                    deletePaymentMethod(
                        id: $paymentId
                    )
                }
           `),
            variables: {
                id: paymentId
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all'
        }
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
    let result: RequestResponse<PaymentMethod[]> = new RequestResponse<PaymentMethod[]>();

    let response: any = await apollo.query(
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
                }`
            ),
            variables: {
                projectId: projectId,
            },
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
