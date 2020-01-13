// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-choice-container" id="projectDropdown">
        <div class="project-selection-overflow-container">
            <div class="project-selection-overflow-container__project-choice" @click="onProjectSelected(project.id)" v-for="project in projects" :key="project.id" >
                <div class="project-selection-overflow-container__project-choice__mark-container">
                    <ProjectSelectionIcon
                        class="project-selection-overflow-container__project-choice__mark-container__image"
                        v-if="project.isSelected"
                    />
                </div>
                <h2 class="project-selection-overflow-container__project-choice__unselected" :class="{'selected': project.isSelected}">{{project.name}}</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelectionIcon from '@/../static/images/header/projectSelection.svg';

import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { PROJECT_USAGE_ACTIONS } from '@/store/modules/usage';
import { Project } from '@/types/projects';
import {
    API_KEYS_ACTIONS,
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

    public async onProjectSelected(projectID: string): Promise<void> {
        this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
        this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');

        try {
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
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

    public get projects(): Project[] {
        return this.$store.getters.projects;
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin-top: 0;
    }

    .project-selection-choice-container {
        position: absolute;
        top: 9vh;
        left: -5px;
        border-radius: 4px;
        padding: 10px 0 10px 0;
        box-shadow: 0 4px rgba(231, 232, 238, 0.6);
        background-color: #fff;
        z-index: 1120;
    }

    .project-selection-overflow-container {
        position: relative;
        min-width: 226px;
        width: auto;
        overflow-y: auto;
        overflow-x: hidden;
        height: auto;
        max-height: 240px;
        background-color: #fff;
        font-family: 'font_regular', sans-serif;

        &__project-choice {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;
            padding-left: 20px;
            padding-right: 20px;

            &__unselected {
                margin: 12px 20px;
                font-size: 14px;
                line-height: 20px;
                color: #354049;
            }

            &:hover {
                background-color: #f2f2f6;
            }

            &__mark-container {
                width: 10px;

                &__image {
                    object-fit: cover;
                }
            }
        }
    }

    .selected {
        font-family: 'font_bold', sans-serif;
    }

    /* width */

    ::-webkit-scrollbar {
        width: 4px;
    }

    /* Track */

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff;
    }

    /* Handle */

    ::-webkit-scrollbar-thumb {
        background: #afb7c1;
        border-radius: 6px;
        height: 5px;
    }

    @media screen and (max-width: 1024px) {

        .project-selection-choice-container {
            top: 50px;
        }
    }
</style>
