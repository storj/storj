// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component } from 'vue';

export class NavigationLink {
    private readonly _path: string;
    private readonly _name: string;
    private readonly _icon: Component | undefined;

    public constructor(path: string, name: string, icon?: Component) {
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

    public get icon(): Component | undefined {
        return this._icon;
    }

    public isChild(): boolean {
        return this._path[0] !== '/';
    }

    public withIcon(icon: Component): NavigationLink {
        return new NavigationLink(this._path, this._name, icon);
    }

    public with(child: NavigationLink): NavigationLink {
        if (!child.isChild()) {
            throw new Error('provided child root is not defined');
        }

        return new NavigationLink(`${this.path}/${child.path}`, child.name);
    }
}
