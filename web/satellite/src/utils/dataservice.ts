// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { getToken } from '../utils/tokenManager';

/**
 * Implementation for HTTP GET requests
 * @param {} url 
 * @param {} auth - indicates if authorithation required 
 */
export async function httpGet(url : string, auth: boolean) : Promise<Answer> {
    let response = await fetch(_createRequest(url, 'GET', auth, null));

    if (response.ok) {
        return response.json();
    }

    return processResponseError(response);
}

/**
 * Implementation for HTTP POST requests
 * @param {} url
 * @param {} data - data to post
 * @param {} auth - indicates if authorithation required
 */
export async function httpPost(url : string, data : any, auth : boolean) : Promise<Answer> {
    let response = await fetch(_createRequest(url, 'POST', auth, data));

    if (response.ok) {
        return await response.json();
    }

    return processResponseError(response);
}

/**
 * Implementation for HTTP PUT requests
 * @param {} url
 * @param {} data - data to post
 * @param {} auth - indicates if authorithation required
 */
export async function httpPut(url : string, data : any, auth : boolean) : Promise<Answer> {
    let response = await fetch(_createRequest(url, 'PUT', auth, data));

    if (response.ok) {
        return await response.json();
    }

    return processResponseError(response);
}

/**
 * Implementation for HTTP DELETE requests
 * @param {} url
 * @param {} data - data to post
 * @param {} auth - indicates if authorithation required
 */
export async function httpDelete(url : string, data : any, auth : boolean) : Promise<Answer> {
    let response = await fetch(_createRequest(url, 'DELETE', auth, data));

    if (response.ok) {
        return await response.json();
    }

    return processResponseError(response);
}

export async function sendForm(url : string, data : any) : Promise<Answer> {
    let params = {
        method: 'POST',
        headers: {
            'Authorization': "Bearer " + getToken()
        },
        body: data
    };
   
    let response = await fetch(new Request(url, params));

    if (response.ok) {
        return await response.json();
    }

    return processResponseError(response);
}

/**
 * Implementation for HTTP GET requests for file downloading via XHR.
 * Only for authorized users.
 * @param {} url
 * @param {} callback - success callback
 * @param {} errorCallback - error callback
 */
export function downloadFile(url : string, callback : any, errorCallback: any) : void {
    var xhr = new XMLHttpRequest();
    xhr.open("GET", url, true);
    xhr.responseType = "blob";
    xhr.setRequestHeader("Content-type", "application/json; charset=utf-8");
    xhr.setRequestHeader("Authorization", "Bearer " + getToken());
    
    xhr.onload = function () {
        if (xhr.readyState == XMLHttpRequest.DONE && xhr.status == 200) {
            console.log(this)
            console.log(this.response)

            callback(this.response);
        } else {
            errorCallback(this.response);
        }
    };

    xhr.send(null);
}

function processResponseError(response: any) {
    switch (response.status) {
        case 401:
            window.location.href = '/login';
        default:
            return { isSuccess: false, error: { code: response.status, message: response.statusText } };
    }
}

/**
 * Creates Request object
 */
function _createRequest(url : string, methodType : string, auth : boolean, data : any) {
    let params = {
        method: methodType,
        headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
            'Connection': 'keep-alive'
        },
    };

    // if (auth) {
    //     params.headers['Authorization'] = "Bearer " + getToken();
    // }

    // if (['PATCH', 'POST', 'PUT', 'DELETE'].includes(methodType) && data) {
    //     params.body = JSON.stringify(data);
    // }

    return new Request(url, params);
}