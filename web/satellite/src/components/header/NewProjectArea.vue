// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <div
            v-if="!isButtonHidden"
            class="new-project-button-container"
            :class="{ active: !hasProjects }"
            @click="toggleSelection"
            id="newProjectButton"
        >
            <h1 class="new-project-button-container__label">+ New Project</h1>
        </div>
        <NewProjectPopup
            v-if="isPopupShown"
            @hideNewProjectButton="hideButton"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NewProjectPopup from '@/components/project/NewProjectPopup.vue';

import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        NewProjectPopup,
    },
})
export default class NewProjectArea extends Vue {
    public isButtonHidden: boolean = false;

    // TODO: temporary solution. Remove when user will be able to create more then one project
    /**
     * Life cycle hook after initial render.
     * Toggles new project button visibility depending on user having his own project or not
     */
    public mounted(): void {
        this.isButtonHidden = this.$store.state.projectsModule.projects.some((project: Project) => {
            return project.ownerId === this.$store.getters.user.id;
        });
    }

    /**
     * Hides new project button
     */
    public hideButton(): void {
        this.isButtonHidden = true;
    }

    /**
     * Opens new project creation popup.
     */
    public toggleSelection(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
    }

    /**
     * Indicates if new project creation popup should be rendered.
     */
    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isNewProjectPopupShown;
    }

    /**
     * Indicates in user has no projects for button highlighting.
     */
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
