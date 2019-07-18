// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// TeamMember stores needed info about user info to show it on UI
import { User } from '@/types/users';

export class TeamMember {
    public user: User;

    public joinedAt: string;
    public isSelected: boolean;

    public constructor(fullName: string, shortName: string, email: string, joinedAt: string, id?: string) {
        this.user = new User(fullName, shortName, email);
        this.user.id = id || '';
        this.joinedAt = joinedAt;
    }

    public formattedFullName(): string {
        let fullName: string = this.user.getFullName();

        if (fullName.length > 16) {
            fullName = fullName.slice(0, 13) + '...';
        }

        return fullName;
    }

    public formattedEmail(): string {
        let email: string = this.user.email;

        if (email.length > 16) {
            email = this.user.email.slice(0, 13) + '...';
        }

        return email;
    }

    public joinedAtLocal(): string {
        if (!this.joinedAt) return '';

        return new Date(this.joinedAt).toLocaleDateString();
    }
}
