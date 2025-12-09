// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * OptionClick defines on click callback type for VDropdown Option.
 */
export type OptionClick = (id?: string) => Promise<void>;

/**
 * Option is a representation of VDropdown item.
 */
export class Option {
    public constructor(
        public label: string = 'no options',
        public onClick: OptionClick = async (_id) => Promise.resolve(),
    ) {}
}

export class NavigationLink {
    constructor(
        public name: string,
        public path: string,
        public icon: string = '',
    ) {}
}
