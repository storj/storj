// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// APIKeyInfo describing api key model in the database
export class ApiKey {
    public id: string;
    public secret: string;
    public name: string;
    public createdAt: string;
    public isSelected: boolean = false;

    constructor(id: string, name: string, createdAt: string, secret: string) {
        this.id = id || '';
        this.name = name || '';
        this.createdAt = createdAt || '';
        this.secret = secret || '';

        this.isSelected = false;
    }

    public formattedName(): string {
        let name = this.name;

        if (name.length < 12) {
            return name;
        }

        return name.slice(0, 12) + '...';
    }

    public getDate(): string {
        if (!this.createdAt) {
            return '';
        }

        return new Date(this.createdAt).toLocaleDateString();
    }
}
