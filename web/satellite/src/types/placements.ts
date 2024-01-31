// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Placement class holds placement information.
 */
export class Placement {
    public constructor(
        public defaultPlacement: number = 0,
        public location: string = '',
    ) { }
}