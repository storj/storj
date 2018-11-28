// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

export function validateEmail(email: string) : boolean {
    const rgx = /^(([^<>()\[\]\\.,;:\s@"]+(\.[^<>()\[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9]+\.)+[a-zA-Z]{2,}))$/;

    return rgx.test(email);
}