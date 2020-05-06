// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-details">
        <div class="project-details__title-area">
            <h1 class="project-details__title-area__title">{{ name }}</h1>
            <span class="project-details__title-area__edit" v-if="!isEditing" @click="toggleEditing">
                Edit Description
            </span>
        </div>
        <p class="project-details__description" v-if="!isEditing">{{ displayedDescription }}</p>
        <div class="project-details__editing" v-else>
            <input
                class="project-details__editing__input"
                placeholder="Enter a description for your project. Descriptions are limited to 100 characters."
                @input="onInput"
                @change="onInput"
                v-model="value"
            />
            <span class="project-details__editing__limit">{{value.length}}/{{MAX_SYMBOLS}}</span>
            <VButton
                class="project-details__editing__cancel-button"
                label="Cancel"
                width="73px"
                height="33px"
                :on-press="toggleEditing"
                is-white="true"
            />
            <VButton
                class="project-details__editing__save-button"
                label="Save"
                width="75px"
                height="35px"
                :on-press="onSaveButtonClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { UpdateProjectModel } from '@/types/projects';

@Component({
    components: {
        VButton,
        HeaderedInput,
    },
})
export default class ProjectDetails extends Vue {
    public readonly MAX_SYMBOLS: number = 100;

    public isEditing: boolean = false;
    public value: string = '';

    /**
     * Returns selected project name.
     */
    public get name(): string {
        return this.$store.getters.selectedProject.name;
    }

    /**
     * Returns selected project description from store.
     */
    public get storedDescription(): string {
        return this.$store.getters.selectedProject.description;
    }

    /**
     * Returns displayed project description on UI.
     */
    public get displayedDescription(): string {
        return this.storedDescription ?
            this.storedDescription :
            'No description yet. Please enter some information about the project if any.';
    }

    /**
     * Triggers on input.
     */
    public onInput({ target }): void {
        if (target.value.length < this.MAX_SYMBOLS) {
            this.value = target.value;

            return;
        }

        this.value = target.value.slice(0, this.MAX_SYMBOLS);
    }

    /**
     * Updates project description.
     */
    public async onSaveButtonClick(): Promise<void> {
        try {
            const updatedProject = new UpdateProjectModel(this.$store.getters.selectedProject.id, this.value);
            await this.$store.dispatch(PROJECTS_ACTIONS.UPDATE, updatedProject);
        } catch (error) {
            await this.$notify.error(`Unable to update project description. ${error.message}`);

            return;
        }

        this.toggleEditing();
        await this.$notify.success('Project updated successfully!');
    }

    /**
     * Toggles project description editing state.
     */
    public toggleEditing(): void {
        this.isEditing = !this.isEditing;
        this.value = this.storedDescription;
    }
}
</script>

<style scoped lang="scss">
    h1,
    p {
        margin: 0;
    }

    .project-details {
        padding: 35px;
        width: calc(100% - 70px);
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        border-radius: 6px;
        margin-bottom: 36px;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-size: 22px;
                line-height: 27px;
                color: #000;
                word-break: break-all;
            }

            &__edit {
                font-size: 14px;
                line-height: 14px;
                color: #909ba8;
                cursor: pointer;

                &:hover {
                    color: #2683ff;
                }
            }
        }

        &__description {
            margin-top: 30px;
            font-size: 16px;
            line-height: 24px;
            color: #354049;
            width: available;
            overflow-y: scroll;
            word-break: break-word;
            max-height: 100px;
        }

        &__editing {
            display: flex;
            align-items: center;
            margin-top: 20px;
            width: calc(100% - 7px);
            border-radius: 6px;
            background-color: #f5f6fa;
            padding-right: 7px;

            &__input {
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                flex: 1;
                height: 48px;
                width: available;
                text-indent: 20px;
                background-color: #f5f6fa;
                border-color: #f5f6fa;
                border-radius: 6px;
            }

            &__limit {
                font-size: 14px;
                line-height: 21px;
                color: rgba(0, 0, 0, 0.3);
                margin: 0 15px;
            }

            &__save-button {
                margin-left: 10px;
            }
        }
    }
</style>
