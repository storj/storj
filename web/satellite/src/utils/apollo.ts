// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { InMemoryCache } from 'apollo-cache-inmemory';
import ApolloClient from 'apollo-client/ApolloClient';
import { ApolloLink } from 'apollo-link';
import { setContext } from 'apollo-link-context';
import { onError } from 'apollo-link-error';
import { HttpLink } from 'apollo-link-http';
import { ServerError } from 'apollo-link-http-common';
import Vue from 'vue';

import { AuthHttpApi } from '@/api/auth';

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
 * Handling unauthorized error.
 */
const errorLink = onError(({ networkError }) => {
    if (!(networkError && (networkError as ServerError).statusCode === 401)) {
        return;
    }

    new AuthHttpApi().logout();
    Vue.prototype.$notify.error('Session token expired');
    setTimeout(() => {
        window.location.href = window.location.origin + '/login';
    }, 3000);
});

/**
 * Combining error and satellite urls.
 */
const link = ApolloLink.from([
    errorLink,
    authLink.concat(satelliteUrl),
]);

/**
 * Creating apollo client.
 */
export const apollo = new ApolloClient({
    link: link,
    cache: new InMemoryCache(),
    connectToDevTools: true,
});
