// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * LoadScript is an utility for loading scripts.
 */
export class LoadScript {
    public readonly head : HTMLHeadElement = document.head;
    public readonly script: HTMLScriptElement = document.createElement('script');

    /**
     * Create script element with some predefined attributes, appends it to a DOM and start loading script.
     * @param url - script url.
     * @param onSuccess - this callback will be fired when load finished.
     * @param onError - this callback will be fired when error occurred.
     */
    public constructor(url: string, onSuccess: LoadScriptOnSuccessCallback, onError: LoadScriptOnErrorCallback) {
        this.head = document.head;
        this.script = document.createElement('script');

        this.script.type = 'text/javascript';
        this.script.charset = 'utf8';
        this.script.async = true;
        this.script.src = url;

        this.script.onload = () => {
            this.script.onerror = null;
            onSuccess();
        };
        this.script.onerror = () => {
            this.script.onerror = null;
            onError(new Error('Failed to load ' + this.script.src));
        };

        this.head.appendChild(this.script);
    }
}

/**
 * LoadScriptOnSuccessCallback describes signature of onSuccess callback.
 */
export type LoadScriptOnSuccessCallback = () => void;

/**
 * LoadScriptOnErrorCallback describes signature of onError callback.
 * @param err - error occurred during script loading.
 */
export type LoadScriptOnErrorCallback = (err: Error) => { };
