// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { AuthToken } from '@/utils/authToken';

export class BaseRest {
    protected async sendRequest (method: string, path: string, body: any = null): Promise<any> {
        // get the authentication token from local storage if it exists
        const token = AuthToken.get();
        const path1 = 'http://localhost:10002' + path;

        const response =  await fetch(path1, {
            method:method,
            body:body,
            headers: {
                authorization: token ? `Bearer ${token}` : '',
                'Content-Type': 'application/json'
            }
        });

        if (!response.ok) {
            const errorMessage = await response.text();
            throw new Error(errorMessage);
        }

        return response;
    }
}
