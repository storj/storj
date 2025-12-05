// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationLink {
    private readonly _path: string;
    private readonly _name: string;
    private readonly _icon: string | undefined;

    public constructor(path: string, name: string, icon?: string) {
        this._path = path;
        this._name = name;
        this._icon = icon;
    }

    public get path(): string {
        return this._path;
    }

    public get name(): string {
        return this._name;
    }

    public get icon(): string | undefined {
        return this._icon;
    }

    public isChild(): boolean {
        return this._path[0] !== '/';
    }

    public with(child: NavigationLink): NavigationLink {
        if (!child.isChild()) {
            throw new Error('provided child root is not defined');
        }

        return new NavigationLink(`${this.path}/${child.path}`, child.name);
    }
}
