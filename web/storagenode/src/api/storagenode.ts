// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.


/**
 * Implementation for HTTP GET requests
 * @param {string} url
 */
export async function httpGet(url) {
    let response = await fetch(url);

    if (response.ok) {
        return response.json();
    }

    return processResponseError(response);
}

function processResponseError(response) {
    switch (response.status) {
        case 401:
            window.location.href = '/login';
        default:
            return { isSuccess: false, error: { code: response.status, message: response.statusText } };
    }
}
