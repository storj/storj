// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export class User {
    public id: string;
    public fullName: string;
    public shortName: string;
    public email: string;
    public partnerId?: string;

    public constructor(id: string = '', fullName: string = '', shortName: string = '', email: string = '', partnerId: string = '') {
        this.id = id;
        this.fullName = fullName;
        this.shortName = shortName;
        this.email = email;
        this.partnerId = partnerId;
    }

    public getFullName(): string {
        return this.shortName === '' ? this.fullName : this.shortName;
    }
}

export class UpdatedUser {
    public fullName: string;
    public shortName: string;

    public constructor(fullName: string = '', shortName: string = '') {
        this.fullName = fullName;
        this.shortName = shortName;
    }
}

// Used in users module to pass parameters to action
export class UpdatePasswordModel {
    public oldPassword: string;
    public newPassword: string;

    public constructor(oldPassword: string = '', newPassword: string = '') {
        this.oldPassword = oldPassword;
        this.newPassword = newPassword;
    }
}
