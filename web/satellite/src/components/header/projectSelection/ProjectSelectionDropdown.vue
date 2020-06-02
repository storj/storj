// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-choice-container" id="projectDropdown">
        <div class="project-selection-overflow-container">
            <div class="project-selection-overflow-container__project-choice" @click="closeDropdown">
                <div class="project-selection-overflow-container__project-choice__mark-container">
                    <ProjectSelectionIcon
                        class="project-selection-overflow-container__project-choice__mark-container__image"
                    />
                </div>
                <p class="project-selection-overflow-container__project-choice__selected">
                    {{ selectedProject.name }}
                </p>
            </div>
            <div
                class="project-selection-overflow-container__project-choice"
                @click="onProjectSelected(project.id)"
                v-for="project in projects"
                :key="project.id"
            >
                <p class="project-selection-overflow-container__project-choice__unselected">{{ project.name }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelectionIcon from '@/../static/images/header/projectSelection.svg';

import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import {
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';

@Component({
    components: {
        ProjectSelectionIcon,
    },
})
export default class ProjectSelectionDropdown extends Vue {
    private FIRST_PAGE = 1;

    /**
     * Fetches all project related information.
     * @param projectID
     */
    public async onProjectSelected(projectID: string): Promise<void> {
        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
        this.closeDropdown();

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }

        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        try {
            await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch api keys. ${error.message}`);
        }

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error('Unable to fetch buckets: ' + error.message);
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project limits. ${error.message}`);
        }
    }

    /**
     * Returns projects list from store.
     */
    public get projects(): Project[] {
        return this.$store.getters.projectsWithoutSelected;
    }

    /**
     * Returns selected project from store.
     */
    public get selectedProject(): Project {
        return this.$store.getters.selectedProject;
    }

    /**
     * Closes dropdown.
     */
    public closeDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin: 0;
        width: 0;
    }

    .project-selection-choice-container {
        position: absolute;
        top: 65px;
        left: -5px;
        background-color: #fff;
        z-index: 1120;
        border: 1px solid #c5cbdb;
        box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
        border-radius: 6px;
    }

    .project-selection-overflow-container {
        position: relative;
        min-width: 226px;
        width: auto;
        overflow-y: scroll;
        height: auto;
        max-height: 240px;
        background-color: #fff;
        font-family: 'font_regular', sans-serif;
        border-radius: 6px;

        &__project-choice {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            padding: 0 25px;

            &__selected,
            &__unselected {
                margin: 12px 0;
                font-size: 14px;
                line-height: 20px;
                color: #7e8b9c;
                word-break: break-all;
            }

            &__selected {
                font-family: 'font_bold', sans-serif;
                color: #494949;
            }

            &:hover {
                background-color: #f2f2f6;

                .project-selection-overflow-container__project-choice__unselected {
                    color: #354049;
                }
            }

            &__mark-container {
                width: 10px;
                margin-right: 12px;

                &__image {
                    object-fit: cover;
                }
            }
        }
    }

    .arrow {
        padding-right: 25px;
    }

    @media screen and (max-width: 1280px) {

        .project-selection-choice-container {
            top: 50px;
        }
    }
</style>
