// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationLink {
    private readonly _path: string;
    private readonly _name: string;

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
