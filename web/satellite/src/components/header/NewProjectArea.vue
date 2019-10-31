// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <div class="new-project-button-container" :class="{ active: !hasProjects }" @click="toggleSelection" id="newProjectButton">
            <h1 class="new-project-button-container__label">+ New Project</h1>
        </div>
        <NewProjectPopup v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NewProjectPopup from '@/components/project/NewProjectPopup.vue';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        NewProjectPopup,
    },
})
export default class NewProjectArea extends Vue {
    public toggleSelection(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isNewProjectPopupShown;
    }

    public get hasProjects(): boolean {
        return this.$store.state.projectsModule.projects.length;
    }
}
</script>

<style scoped lang="scss">
    .new-project-container {
        background-color: #fff;
    }

    .new-project-button-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        width: 170px;
        height: 50px;
        border-radius: 6px;
        border: 1px solid #afb7c1;
        background-color: white;
        cursor: pointer;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }

        &:hover {
            background-color: #2683ff;
            border: 1px solid #2683ff;
            box-shadow: 0 4px 20px rgba(35, 121, 236, 0.4);

            .new-project-button-container__label {
                color: white;
            }
        }
    }

    .new-project-button-container.active {
        background-color: #2683ff;
        border: 1px solid #2683ff;
        box-shadow: 0 4px 20px rgba(35, 121, 236, 0.4);

        .new-project-button-container__label {
            color: white;
        }

        &:hover {
            box-shadow: none;
        }
    }
</style>
