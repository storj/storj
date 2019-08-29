// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * NavigationLink class holds info for NavigationLink entity.
 */
export class NavigationLink {
    private _path: string;
    private _name: string;

    public constructor(path: string, name: string) {
        this._path = path;
        this._name = name;
    }

    public get path(): string {
        return this._path;
    }

    public get name(): string {
        return this._name;
    }
}
