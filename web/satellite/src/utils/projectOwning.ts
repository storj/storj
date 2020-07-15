// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { Store } from 'vuex';

import { Project } from '@/types/projects';

/**
 * ProjectOwning exposes method that returns user's projects amount.
 */
export class ProjectOwning {
    public constructor(public store: Store<any>) {}

    public usersProjectsCount(): number {
        let projectsCount: number = 0;

        this.store.state.projectsModule.projects.map((project: Project) => {
            if (project.ownerId === this.store.getters.user.id) {
                projectsCount++;
            }
        });

        return projectsCount;
    }
}
