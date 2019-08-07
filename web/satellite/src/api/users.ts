// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { UpdatedUser, User } from '@/types/users';
import { RequestResponse } from '@/types/response';

// Performs update user info graphQL mutation request.
// Returns User object if succeed, null otherwise
export async function updateAccountRequest(user: UpdatedUser): Promise<RequestResponse<UpdatedUser>> {
    let result: RequestResponse<UpdatedUser> = new RequestResponse<UpdatedUser>();

    let response: any = await apolloManager.mutate(
        {
            mutation: gql(`
                mutation {
                    updateAccount (
                        input: {
                            fullName: $fullName,
                            shortName: $shortName
                        }
                    ) {
                        fullName,
                        shortName
                    }
                }`,
            ),
            variables: {
                fullName: user.fullName,
                shortName: user.shortName,
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = new UpdatedUser(
            response.data.updateAccount.fullName,
            response.data.updateAccount.shortName
        );
    }

    return result;
}

// Performs change password graphQL mutation
// Returns base user fields
export async function changePasswordRequest(password: string, newPassword: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response: any = await apolloManager.mutate(
        {
            mutation: gql(`
                mutation($password: String!, $newPassword: String!) {
                    changePassword (
                        password: $password,
                        newPassword: $newPassword
                    ) {
                       email
                    }
                }`
            ),
            variables: {
                password: password,
                newPassword: newPassword
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

export async function forgotPasswordRequest(email: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response: any = await apolloManager.query(
        {
            query: gql(`
                query($email: String!) {
                    forgotPassword(email: $email)
                }`),
            variables: {
                email: email
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

// Performs Create user graqhQL request.
export async function createUserRequest(user: User, password: string, secret: string, refUserId?: string): Promise<RequestResponse<string>> {
    let result: RequestResponse<string> = new RequestResponse<string>();

    let response = await apolloManager.mutate(
        {
            mutation: gql(`
                mutation($email: String!, $password: String!, $fullName: String!, $shortName: String!, $partnerID: String!, $referrerUserID: String!, $secret: String!) {
                    createUser(
                        input:{
                            email: $email,
                            password: $password,
                            fullName: $fullName,
                            shortName: $shortName,
                            partnerId: $partnerID
                        },
                        referrerUserId: $referrerUserID,
                        secret: $secret,
                    ){email, id}
                }`
            ),
            variables: {
                email: user.email,
                password: password,
                fullName: user.fullName,
                shortName: user.shortName,
                partnerID: user.partnerId ? user.partnerId : '',
                referrerUserID: refUserId ? refUserId : '',
                secret: secret
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        if (response.data) {
            result.data = (response as any).data.createUser.id;
        }
    }

    return result;
}

// Performs graqhQL request.
// Returns Token.
export async function getTokenRequest(email: string, password: string): Promise<RequestResponse<string>> {
    let result: RequestResponse<string> = new RequestResponse<string>();

    let response: any = await apolloManager.query(
        {
            query: gql(`
                query ($email: String!, $password: String!) { 
                    token(email: $email, password: $password) {
                        token
                    }
                }`
            ),
            variables: {
                email: email,
                password: password
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.token.token;
    }

    return result;
}

// Performs graqhQL request.
// Returns User object.
export async function getUserRequest(): Promise<RequestResponse<User>> {
    let result: RequestResponse<User> = new RequestResponse<User>();

    let response: any = await apolloManager.query(
        {
            query: gql(`
                query {
                    user {
                        id,
                        fullName,
                        shortName,
                        email,
                        partnerId,
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
        result.data = new User(
            response.data.user.id,
            response.data.user.fullName,
            response.data.user.shortName,
            response.data.user.email,
            response.data.user.partnerId
        );
    }

    return result;
}

// Performs graqhQL request.
export async function deleteAccountRequest(password: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response = await apolloManager.mutate(
        {
            mutation: gql(`
                mutation ($password: String!){
                    deleteAccount(password: $password) {
                        email
                    }
                }`
            ),
            variables: {
                password: password
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

export async function resendEmailRequest(userID: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = new RequestResponse<null>();

    let response = await apolloManager.query(
        {
            query: gql(`
                query ($userID: String!){
                    resendAccountActivationEmail(id: $userID)
                }`
            ),
            variables: {
                userID: userID
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}
