// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-container" id="projectDropdownButton">
        <div class="project-selection-toggle-container" v-on:click="toggleSelection">
            <h1>{{name}}</h1>
            <div class="project-selection-toggle-container__expander-area">
                <img v-if="!isDropdownShown" src="../../../../static/images/register/BlueExpand.svg"/>
                <img v-if="isDropdownShown" src="../../../../static/images/register/BlueHide.svg"/>
            </div>
        </div>
        <ProjectSelectionDropdown v-if="isDropdownShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { mapState } from 'vuex';
import ProjectSelectionDropdown from './ProjectSelectionDropdown.vue';
import { APP_STATE_ACTIONS, PROJETS_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        methods: {
            toggleSelection: async function (): Promise<any> {
                const response = await this.$store.dispatch(PROJETS_ACTIONS.FETCH);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
            }
        },
        computed: mapState({
            name: (state: any): string => {
                let selectedProject = state.projectsModule.selectedProject;

                return selectedProject.id ? selectedProject.name : 'Choose project';
            },
            isDropdownShown: (state: any) => state.appStateModule.appState.isProjectsDropdownShown
        }),
        components: {
            ProjectSelectionDropdown
        }
    }
)

export default class ProjectSelectionArea extends Vue {
}
</script>

<style scoped lang="scss">
    .project-selection-container {
        position: relative;
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
        cursor: pointer;

        h1 {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }

        &:hover {

            h1 {
                opacity: 0.7;
            }
        }
    }

    .project-selection-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 100%;
        height: 50px;

        h1 {
            transition: opacity .2s ease-in-out;
        }

        &__expander-area {
            margin-left: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 28px;
            height: 28px;
        }
    }
</style>