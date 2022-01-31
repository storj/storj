// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * NavigationLink class holds info for NavigationLink entity.
 */
export class NavigationLink {
    public readonly path: string;
    public readonly name: string;

    public constructor(path: string, name: string) {
        this.path = path;
        this.name = name;
    }
}
