// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { InMemoryCache } from 'apollo-cache-inmemory';
import ApolloClient from 'apollo-client/ApolloClient';
import { setContext } from 'apollo-link-context';
import { HttpLink } from 'apollo-link-http';

import { AuthToken } from '@/utils/authToken';

// Satellite url
const satelliteUrl = new HttpLink({
    uri: process.env.VUE_APP_ENDPOINT_URL,
});

// Adding auth headers
const authLink = setContext((_, {headers}) => {
    // get the authentication token from local storage if it exists
    const token = AuthToken.get();

    // return the headers to the context so httpLink can read them
    return {
        headers: {
            ...headers,
            authorization: token ? `Bearer ${token}` : '',
        }
    };
});

// Creating apollo client
export default new ApolloClient({
    link: authLink.concat(satelliteUrl),
    cache: new InMemoryCache(),
    connectToDevTools: true,
});
