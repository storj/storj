// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationLink {
    public path: string;
    public name: string;

    public constructor(path: string, name: string) {
        this.path = path;
        this.name = name;
    }

}
