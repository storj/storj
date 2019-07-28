// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="project-details">
            <h1>Project Details</h1>
            <div class="project-details-info-container">
                <div class="project-details-info-container__name-container">
                    <h2>Project Name</h2>
                    <h3>{{name}}</h3>
                </div>
            </div>
            <div class="project-details-info-container">
                <div class="project-details-info-container__description-container" v-if="!isEditing">
                    <div class="project-details-info-container__description-container__text">
                        <h2>Description</h2>
                        <h3>{{description}}</h3>
                    </div>
                    <div title="Edit">
                        <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="toggleEditing">
                            <rect width="40" height="40" rx="4" fill="#E2ECF7"/>
                            <path d="M19.0901 21.4605C19.3416 21.7259 19.6695 21.8576 19.9995 21.8576C20.3295 21.8576 20.6574 21.7259 20.9089 21.4605L28.6228 13.3181C29.1257 12.7871 29.1257 11.9291 28.6228 11.3982C28.1198 10.8673 27.3069 10.8673 26.8039 11.3982L19.0901 19.5406C18.5891 20.0715 18.5891 20.9295 19.0901 21.4605ZM27.7134 19.1435C27.0031 19.1435 26.4277 19.7509 26.4277 20.5005V27.2859H13.5713V13.7152H19.9995C20.7097 13.7152 21.2851 13.1078 21.2851 12.3581C21.2851 11.6085 20.7097 11.0011 19.9995 11.0011H13.5713C12.1508 11.0011 11 12.2158 11 13.7152V27.2859C11 28.7852 12.1508 30 13.5713 30H26.4277C27.8482 30 28.999 28.7852 28.999 27.2859V20.5005C28.999 19.7509 28.4236 19.1435 27.7134 19.1435Z" fill="#2683FF"/>
                        </svg>
                    </div>
                </div>
                <div class="project-details-info-container__description-container--editing" v-if="isEditing">
                    <HeaderedInput
                        label="Description"
                        placeholder="Enter Description"
                        width="70vw"
                        height="10vh"
                        isMultiline="true"
                        @setData="setNewDescription" />
                    <div class="project-details-info-container__description-container__buttons-area">
                        <Button label="Cancel" width="180px" height="48px" :onPress="toggleEditing" isWhite="true"/>
                        <Button label="Save" width="180px" height="48px" :onPress="onSaveButtonClick"/>
                    </div>
                </div>
            </div>
            <div class="project-details__button-area" id="deleteProjectPopupButton">
                <Button class="delete-project" label="Delete project" width="180px" height="48px" :onPress="toggleDeleteDialog" isDeletion="true"/>
            </div>
        </div>
        <DeleteProjectPopup v-if="isPopupShown" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import HeaderedInput from '@/components/common/HeaderedInput.vue';
    import Checkbox from '@/components/common/Checkbox.vue';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import { PROJETS_ACTIONS, APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import ROUTES from '@/utils/constants/routerConstants';
    import DeleteProjectPopup from '@/components/project/DeleteProjectPopup.vue';

    @Component({
        components: {
            Button,
            HeaderedInput,
            Checkbox,
            EmptyState,
            DeleteProjectPopup,
        }
    })
    export default class ProjectDetailsArea extends Vue {
        private isEditing: boolean = false;
        private newDescription: string = '';

        public mounted(): void {
            this.$store.dispatch(PROJETS_ACTIONS.FETCH);
        }

        public get name(): string {
            return this.$store.getters.selectedProject.name;
        }

        public get description(): string {
            return this.$store.getters.selectedProject.description ?
                this.$store.getters.selectedProject.description :
                'No description yet. Please enter some information about the project if any.';
        }

        public get isPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isDeleteProjectPopupShown;
        }

        public setNewDescription(value: string): void {
            this.newDescription = value;
        }

        public async onSaveButtonClick(): Promise<any> {
            let response = await this.$store.dispatch(
                PROJETS_ACTIONS.UPDATE, {
                    id: this.$store.getters.selectedProject.id,
                    description: this.newDescription
                }
            );

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                return;
            }

            this.toggleEditing();
            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Project updated successfully!');
        }

        public toggleDeleteDialog(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
        }

        public onMoreClick(): void {
            this.$router.push(ROUTES.USAGE_REPORT);
        }

        private toggleEditing(): void {
            this.isEditing = !this.isEditing;
            // TODO: cache this value in future
            this.newDescription = '';
        }
    }
</script>

<style scoped lang="scss">
    .project-details {
        position: relative;
        overflow: hidden;
        height: 85vh;
        
        h1 {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
        
        h2 {
            @extend h1;
            font-family: 'font_regular';
            font-size: 16px;
            line-height: 21px;
            color: rgba(56, 75, 101, 0.4);
        }
        
        h3 {
            @extend h2;
            color: #354049;
        }
        
        h4 {
            @extend h1;
            font-size: 18px;
            line-height: 27px;
        }
        
        &__terms-area {
            display: flex;
            flex-direction: row;
            justify-content: flex-start;
            align-items: center;
            margin-top: 20px;
            
            img {
                margin-top: 20px;
            }
            
            &__checkbox {
                align-self: center;
            }
            
            h2 {
                font-family: 'font_regular';
                font-size: 14px;
                line-height: 20px;
                margin-top: 30px;
                margin-left: 10px;
            }
        }
        
        &__button-area {
            margin-top: 3vh;
            margin-bottom: 100px;
        }
    }
    
    .project-details-info-container {
        height: auto;
        margin-top: 37px;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: flex-start;
        
        &__name-container {
            min-height: 67px;
            width: 100%;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: flex-start;
            padding: 28px;
            background-color: #fff;
        }
        
        &__description-container {
            @extend .project-details-info-container__name-container;
            min-height: 67px;
            height: auto;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            
            &__text {
                display: flex;
                flex-direction: column;
                justify-content: center;
                align-items: flex-start;
                width: 65vw;
                
                h3 {
                    width: 100%;
                    word-wrap: break-word;
                }
            }
            
            &--editing {
                @extend .project-details-info-container__description-container;
                display: flex;
                flex-direction: column;
                justify-content: center;
                align-items: flex-start;
            }
            
            &__buttons-area {
                margin-top: 2vh;
                display: flex;
                flex-direction: row;
                align-items: center;
                width: 380px;
                justify-content: space-between;
            }
            
            svg {
                cursor: pointer;
                
                &:hover {
	                
                    rect {
                        fill: #2683FF !important;
                    }
	                
                    path {
                        fill: white !important;
                    }
                }
            }
        }
        
        &__portability-container {
            @extend .project-details-info-container__description-container;
            
            &__info {
                display: flex;
                flex-direction: row;
                align-items: center;
            
                &__text {
                    margin-left: 2vw;
                }
            }
            
            &__buttons-area {
                @extend .project-details-info-container__portability-container__info;
                width: 380px;
                justify-content: space-between;
            }
            
            img {
                width: 6vw;
                height: 10vh;
            }
        }
    }
</style>
