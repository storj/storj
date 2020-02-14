// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { InMemoryCache } from 'apollo-cache-inmemory';
import ApolloClient from 'apollo-client/ApolloClient';
import { setContext } from 'apollo-link-context';
import { HttpLink } from 'apollo-link-http';

/**
 * Satellite url.
 */
const satelliteUrl = new HttpLink({
    uri: process.env.VUE_APP_ENDPOINT_URL,
});

/**
 * Adding additional headers.
 */
const authLink = setContext((_, {headers}) => {
    // return the headers to the context so httpLink can read them
    return {
        headers: {
            ...headers,
        },
    };
});

/**
 * Creating apollo client.
 */
export const apollo = new ApolloClient({
    link: authLink.concat(satelliteUrl),
    cache: new InMemoryCache(),
    connectToDevTools: true,
});
