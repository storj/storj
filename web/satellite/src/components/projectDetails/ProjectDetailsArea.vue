// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="project-details" v-if="isProjectSelected">
            <h1>Project Details</h1>
            <div class="project-details-info-container">
                <div class="project-details-info-container__name-container">
                    <h2>Project Name</h2>
                    <h3>{{name}}</h3>
                </div>
                <div class="project-details-info-container__name-container">
                    <h2>Company / Organization</h2>
                    <h3>{{companyName}}</h3>
                </div>
            </div>
            <div class="project-details-info-container">
                <div class="project-details-info-container__description-container" v-if="!isEditing">
                    <div class="project-details-info-container__description-container__text">
                        <h2>Description</h2>
                        <h3>{{description}}</h3>
                    </div>
                    <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="toggleEditing">
                        <rect width="40" height="40" rx="4" fill="#E2ECF7"/>
                        <path d="M19.0901 21.4605C19.3416 21.7259 19.6695 21.8576 19.9995 21.8576C20.3295 21.8576 20.6574 21.7259 20.9089 21.4605L28.6228 13.3181C29.1257 12.7871 29.1257 11.9291 28.6228 11.3982C28.1198 10.8673 27.3069 10.8673 26.8039 11.3982L19.0901 19.5406C18.5891 20.0715 18.5891 20.9295 19.0901 21.4605ZM27.7134 19.1435C27.0031 19.1435 26.4277 19.7509 26.4277 20.5005V27.2859H13.5713V13.7152H19.9995C20.7097 13.7152 21.2851 13.1078 21.2851 12.3581C21.2851 11.6085 20.7097 11.0011 19.9995 11.0011H13.5713C12.1508 11.0011 11 12.2158 11 13.7152V27.2859C11 28.7852 12.1508 30 13.5713 30H26.4277C27.8482 30 28.999 28.7852 28.999 27.2859V20.5005C28.999 19.7509 28.4236 19.1435 27.7134 19.1435Z" fill="#2683FF"/>
                    </svg>
                </div>
                <div class="project-details-info-container__description-container--editing" v-if="isEditing">
                    <HeaderedInput
                        label="Description"
                        placeholder ="Enter Description"
                        width="70vw"
                        height="10vh"
                        isMultiline
                        @setData="setNewDescription" />
                    <div class="project-details-info-container__description-container__buttons-area">
                        <Button label="Cancel" width="10vw" height="5vh" :onPress="toggleEditing" isWhite/>
                        <Button label="Save" width="10vw" height="5vh" :onPress="onSaveButtonClick"/>
                    </div>
                </div>
            </div>
            <div class="project-details-info-container">
                <div class="project-details-info-container__portability-container">
                    <div class="project-details-info-container__portability-container__info">
                        <img src="../../../static/images/projectDetails/Portability.png" alt="">
                        <div class="project-details-info-container__portability-container__info__text">
                            <h4>Data Portability</h4>
                            <h2>Backup project data to recover or move between Satellites</h2>
                        </div>
                    </div>
                    <div class="project-details-info-container__portability-container__buttons-area">
                        <Button label="Export" width="10vw" height="5vh" :onPress="onExportClick" isWhite/>
                        <Button label="Import" width="10vw" height="5vh" :onPress="onImportClick"/>
                    </div>
                </div>
            </div>
            <div class="project-details__terms-area">
                <img src="static/images/projectDetails/checked.svg" alt="">
                <h2>{{projectApproval}}</h2>
            </div>
            <div class="project-details__button-area">
                <!-- TODO: change vw to px -->
                <Button label="Delete project" width="10vw" height="5vh" :onPress="onDeleteButtonClick" isWhite/>
            </div>
        </div>
        <EmptyState 
            v-if="!isProjectSelected"
            mainTitle="Choose or Create new project"
            :imageSource="emptyImage" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import Checkbox from '@/components/common/Checkbox.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages'

@Component(
    { 
        data: function() {
            return {
                isEditing: false,
                newDescription: "",
                emptyImage: EMPTY_STATE_IMAGES.PROJECT
            }
        },
        methods: {
            toggleEditing: function() : void {
                this.$data.isEditing = !this.$data.isEditing;
                // TODO: cache this value in future
                this.$data.newDescription = "";
            },
            setNewDescription: function(value: string): void {
                this.$data.newDescription = value;
            },
            onSaveButtonClick: async function(): Promise<any> {
                let isUpdateSuccess = await this.$store.dispatch(
                    "updateProjectDescription", {
                        id: this.$store.getters.selectedProject.id,
                        description: this.$data.newDescription,
                    }
                );

                isUpdateSuccess
                    // TODO: call toggleEditing method instead of this IIF
                    ? (()=>{
                        this.$data.isEditing = !this.$data.isEditing;
                        this.$data.newDescription = "";
                    })()
                    //TODO: popup error here
                    : console.log("error during project updating!");
            },
            onDeleteButtonClick: async function(): Promise<any> {
                let isDeleteSuccess = await this.$store.dispatch(
                    "deleteProject",
                    this.$store.getters.selectedProject.id,
                );

                if (!isDeleteSuccess) {
                    //TODO: popup error here
                    console.log("error during project deletion!");
                }
            }
        },
        computed: {
            name: function(): string {
                return this.$store.getters.selectedProject.name;
            },
            description: function(): string {
                return this.$store.getters.selectedProject.description;
            },
            companyName: function(): string {
                return this.$store.getters.selectedProject.companyName;
            },
            projectApproval: function(): string {
                let date = new Date(this.$store.getters.selectedProject.createdAt);

                return `Project Approval ${date.toLocaleDateString()} by ${this.$store.getters.selectedProject.ownerName}`;
            },
            // this computed is used to indicate if project is selected.
            // if false - we should change UI
            isProjectSelected: function(): boolean {
                return this.$store.getters.selectedProject.id !== ""
            },
        },
        components: {
            Button,
            HeaderedInput,
            Checkbox,
            EmptyState,
        }
    }
)

export default class ProjectDetailsArea extends Vue {}
</script>

<style scoped lang="scss">
    .project-details {
        padding: 55px 55px 55px 55px;
        position: relative;
        overflow-y: auto;
        overflow-x: hidden;
        height: 85vh;
        h1 {
            font-family: 'montserrat_bold';
			font-size: 24px;
			line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
        h2 {
            @extend h1;
            font-family: 'montserrat_regular';
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
                font-family: 'montserrat_regular';
                font-size: 14px;
                line-height: 20px;
                margin-top: 30px;
                margin-left: 10px;
            }
        }

        &__button-area {
            margin-top: 3vh;
            margin-bottom: 5vh;
        }
    }
    .project-details-info-container {
        height: auto;
        margin-top: 4vh;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: flex-start;

        &__name-container {
            height: 10vh;
            width: 33vw;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: flex-start;
            padding: 3vh 3vh 3vh 3vh;
            background-color: #fff;
            
            &:hover {
                box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
            }
        }

        &__description-container {
            @extend .project-details-info-container__name-container;
            height: auto;
            width: 72vw;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;

            &__text {
                display: flex;
                flex-direction: column;
                justify-content: center;
                align-items: flex-start;
                width: 65vw;
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
                width: 22vw;
                justify-content: space-between;
            }

            svg {
                cursor: pointer;
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
                width: 22vw;
                justify-content: space-between;
            }

            img {
                width: 6vw;
                height: 10vh;
            }
        }
    }
</style>