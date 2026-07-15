// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Parameters accepted by turnstile.render().
 */
export type TurnstileRenderParams = {
    sitekey: string;
    execution?: 'render' | 'execute';
    appearance?: 'always' | 'execute' | 'interaction-only';
    callback?: (token: string) => void;
    'error-callback'?: () => void;
    'expired-callback'?: () => void;
    'timeout-callback'?: () => void;
};

/**
 * The Cloudflare Turnstile client API exposed on the window object.
 */
export type Turnstile = {
    render: (container: string | HTMLElement, params: TurnstileRenderParams) => string;
    execute: (widget: string | HTMLElement) => void;
    reset: (widget?: string | HTMLElement) => void;
    remove: (widget: string | HTMLElement) => void;
    getResponse: (widget?: string | HTMLElement) => string | undefined;
};

declare global {
    interface Window {
        turnstile?: Turnstile;
        onloadTurnstileCallback?: () => void;
    }
}

const SCRIPT_ID = 'cf-turnstile-script';
const SCRIPT_SRC = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit&onload=onloadTurnstileCallback';

let loadPromise: Promise<Turnstile> | null = null;

/**
 * Loads the Cloudflare Turnstile script once and resolves with the client API.
 */
export function loadTurnstile(): Promise<Turnstile> {
    if (loadPromise) {
        return loadPromise;
    }

    loadPromise = new Promise<Turnstile>((resolve, reject) => {
        if (window.turnstile) {
            resolve(window.turnstile);
            return;
        }

        window.onloadTurnstileCallback = () => {
            if (window.turnstile) {
                resolve(window.turnstile);
            } else {
                reject(new Error('Turnstile failed to initialize'));
            }
        };

        const existing = document.getElementById(SCRIPT_ID);
        if (existing) {
            return;
        }

        const script = document.createElement('script');
        script.id = SCRIPT_ID;
        script.src = SCRIPT_SRC;
        script.async = true;
        script.defer = true;
        script.onerror = () => {
            script.remove();
            loadPromise = null;
            reject(new Error('Failed to load Turnstile script'));
        };
        document.head.appendChild(script);
    });

    return loadPromise;
}
