// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

export function useDOM() {
    function removeReadOnly(e): void {
        if (e.target.readOnly) {
            e.target.readOnly = false;
        }
    }

    function addReadOnly(e): void {
        if (!e.target.readOnly) {
            e.target.readOnly = true;
        }
    }

    return {
        removeReadOnly,
        addReadOnly,
    };
}
