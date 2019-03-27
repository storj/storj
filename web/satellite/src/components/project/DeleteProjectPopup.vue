// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-project-popup-container">
        <div class="delete-project-popup" id="deleteProjectPopup">
            <div class="delete-project-popup__info-panel-container">
                <h2 class="delete-project-popup__info-panel-container__main-label-text">Delete Project</h2>
                <div v-html="imageSource"></div>
            </div>
            <div class="delete-project-popup__form-container">
                <p>Are you sure that you want to delete your project? You will lose all your buckets and files that linked to this project.</p>
                <div>
                    <p class="text" v-if="!nameError">To proceed with deletion, enter full project name</p>
                    <div v-if="nameError" class="delete-project-popup__form-container__label">
                        <img src="../../../static/images/register/ErrorInfo.svg"/>
                        <p class="text">{{nameError}}</p>
                    </div>
                    <input 
                        type="text" 
                        placeholder="Enter Project Name"
                        v-model="projectName"
                        v-on:keyup="resetError" >
                </div>
                
                <div class="delete-project-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite/>
                    <Button 
                        label="Delete"
                        width="205px" 
                        height="48px" 
                        class="red"
                        :onPress="onDeleteProjectClick" 
                        :isDisabled="isDeleteButtonDisabled" />
                </div>
            </div>
            <div class="delete-project-popup__close-cross-container">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import { PROJETS_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        data: function () {
            return {
                projectName: '',
                nameError: '',
                imageSource: EMPTY_STATE_IMAGES.DELETE_PROJECT,
            };
        },
        methods: {
            resetError: function (): void {
                this.$data.nameError = '';
            },
            onDeleteProjectClick: async function (): Promise<any> {
                if (this.$data.projectName !== this.$store.getters.selectedProject.name) {
                    this.$data.nameError = 'Name doesn\'t match with current project name';

                    return;
                }

                let response = await this.$store.dispatch(
                    PROJETS_ACTIONS.DELETE,
                    this.$store.getters.selectedProject.id,
                );

                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Error during project deletion');

                    return;
                }

                this.$store.dispatch(PM_ACTIONS.CLEAR);
                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Project was successfully deleted');
                this.$store.dispatch(PROJETS_ACTIONS.FETCH);
                this.$router.push('/');
            },
            onCloseClick: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
            }
        },
        computed: {
            isDeleteButtonDisabled: function (): boolean {
                
                return (this.$data.projectName === '' || this.$data.nameError !== '');
            },
        },
        components: {
            Button
        }
    }
)

export default class DeleteProjectPopup extends Vue {
}
</script>

<style scoped lang="scss">
    .delete-project-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    .input-container.full-input {
        width: 100%;
    }
    .red {
        background-color: #EB5757;
    }
    .delete-project-popup {
        width: 100%;
        max-width: 800px;
        height: 460px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        position: relative;
        justify-content: space-between;
        padding: 20px 100px 0px 100px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 55px;

            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 30px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 440px;
            height: 335px;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 21px;
                margin-bottom: 30px;
            }

            &__label {
                display: flex;
                flex-direction: row;
                align-items: center;

                p {
                    padding-left: 10px;
                    color: #EB5757;
                    margin: 0;
                }
            }

            .text {
                margin: 0px;
            }

            input {
                font-family: 'font_regular';
                font-size: 16px;
                line-height: 21px;
                margin-top: 10px;
                resize: none;
                margin-bottom: 18px;
                height: 48px;
                width: 100%;
                text-indent: 20px;
                border-color: rgba(56, 75, 101, 0.4);
                border-radius: 6px;
                outline: none;
                box-shadow: none;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover svg path {
                fill: #2683FF;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .delete-project-popup {

            &__info-panel-container {
                display: none;

            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>
