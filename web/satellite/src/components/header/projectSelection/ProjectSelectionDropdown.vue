// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-selection-choice-container" id="projectDropdown">
        <div class="project-selection-overflow-container">
            <!-- loop for rendering projects -->
            <!-- TODO: add selection logic onclick -->
            <div class="project-selection-overflow-container__project-choice" v-on:click="onProjectSelected(project.id)" v-for="project in projects" v-bind:key="project.id" >
                <div class="project-selection-overflow-container__project-choice__mark-container">
                    <svg v-if="project.isSelected" width="15" height="13" viewBox="0 0 15 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M14.0928 3.02746C14.6603 2.4239 14.631 1.4746 14.0275 0.907152C13.4239 0.339699 12.4746 0.368972 11.9072 0.972536L14.0928 3.02746ZM4.53846 11L3.44613 12.028C3.72968 12.3293 4.12509 12.5001 4.53884 12.5C4.95258 12.4999 5.34791 12.3289 5.63131 12.0275L4.53846 11ZM3.09234 7.27469C2.52458 6.67141 1.57527 6.64261 0.971991 7.21036C0.36871 7.77812 0.339911 8.72743 0.907664 9.33071L3.09234 7.27469ZM11.9072 0.972536L3.44561 9.97254L5.63131 12.0275L14.0928 3.02746L11.9072 0.972536ZM5.6308 9.97199L3.09234 7.27469L0.907664 9.33071L3.44613 12.028L5.6308 9.97199Z" fill="#2683FF"/>
                    </svg>
                </div>
                <h2 v-bind:class="[project.isSelected ? 'project-selection-overflow-container__project-choice--selected' : 'project-selection-overflow-container__project-choice--unselected']">{{project.name}}</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { APP_STATE_ACTIONS, PROJETS_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS, API_KEYS_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        computed: {
            projects: function () {
                return this.$store.getters.projects;
            }
        },
        methods: {
            onProjectSelected: async function (projectID: string): Promise<void> {
                this.$store.dispatch(PROJETS_ACTIONS.SELECT, projectID);
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PROJECTS);
                this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');

                const pmResponse = await this.$store.dispatch(PM_ACTIONS.FETCH);
                const keysResponse = await this.$store.dispatch(API_KEYS_ACTIONS.FETCH);

                if (!pmResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
                }

                if (!keysResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch api keys');
                }
            }
        },
    }
)

export default class ProjectSelectionDropdown extends Vue {
}
</script>

<style scoped lang="scss">
    .project-selection-choice-container {
        position: absolute;
        top: 9vh;
        left: -5px;
        border-radius: 4px;
        padding: 10px 0px 10px 0px;
        box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        background-color: #FFFFFF;
        z-index: 1120;
    }
    .project-selection-overflow-container {
        position: relative;
        width: 226px;
        overflow-y: auto;
        overflow-x: hidden;
        height: auto;
        max-height: 240px;
        background-color: #FFFFFF;

        &__project-choice {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;
            padding-left: 20px;
            padding-right: 20px;

            h2{
                margin-left: 20px;
                font-size: 14px;
                line-height: 20px;
                color: #354049;
            }

            &:hover {
                background-color: #F2F2F6;
            }

            &--selected {
                font-family: 'font_bold';
            }

            &--unselected {
                font-family: 'font_regular';
            }

            &__mark-container {
                width: 10px;;
                svg {
                    object-fit: cover;
                }
            }
        }
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
        background: #AFB7C1;
        border-radius: 6px;
        height: 5px;
    }
</style>