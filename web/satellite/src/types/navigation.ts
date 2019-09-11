// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationLink {
    private _path: string;
    private _name: string;
    private _icon: string;

    public constructor(path: string, name: string, icon: string = '') {
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
    public get icon(): string {
        return this._icon;
    }

    public isChild(): boolean {
        return this._path[0] !== '/';
    }

    public withIcon(icon: string): NavigationLink {
        return new NavigationLink(this._path, this._name, icon);
    }

    public with(child: NavigationLink): NavigationLink {
        if (!child.isChild()) {
            // TODO: better error message
            throw new Error('child root is not child');
        }

        return new NavigationLink(`${this.path}/${child.path}`, child.name);
    }
}
