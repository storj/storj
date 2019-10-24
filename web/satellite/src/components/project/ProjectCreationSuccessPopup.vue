// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isPopupShown" class="project-creation-success-popup-container">
        <div class="project-creation-success-popup" id='successfulProjectCreationPopup'>
            <div class="project-creation-success-popup__info-panel-container">
                <ProjectCreationSuccessIcon/>
            </div>
            <div class="project-creation-success-popup__form-container">
                <h2 class="project-creation-success-popup__form-container__main-label-text">Congrats!</h2>
                <p class="project-creation-success-popup__form-container__confirmation-text">You just created your project. Next, we recommend you create your first API Key for this project. API Keys allow developers to manage their projects and build applications on top of the Storj network through our
                    <a class="project-creation-success-popup__form-container__confirmation-text__link" href="https://github.com/storj/storj/wiki/Uplink-CLI" target="_blank">Uplink CLI.</a>
                </p>
                <div class="project-creation-success-popup__form-container__button-container">
                    <VButton
                        label="I will do it later"
                        width="214px"
                        height="50px"
                        :on-press="onCloseClick"
                        is-white="true"
                    />
                    <VButton
                        label="Create first API Key"
                        width="214px"
                        height="50px"
                        :on-press="onCreateAPIKeyClick"
                    />
                </div>
            </div>
            <div class="project-creation-success-popup__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import ProjectCreationSuccessIcon from '@/../static/images/project/projectCreationSuccess.svg';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        VButton,
        ProjectCreationSuccessIcon,
        CloseCrossIcon,
    },
})
export default class ProjectCreationSuccessPopup extends Vue {
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP);
    }

    public onCreateAPIKeyClick(): void {
        this.$router.push(RouteConfig.ApiKeys.path);
        this.onCloseClick();
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulProjectCreationPopupShown;
    }
}
</script>

<style scoped lang="scss">
    .project-creation-success-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1000;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    
    .project-creation-success-popup {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px 100px 80px 50px;
        
        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;
            margin-top: 20px;
        }
        
        &__form-container {
            width: 100%;
            max-width: 440px;
            margin-top: 10px;
            
            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin: 0;
            }
            
            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 40px;
            }

            &__confirmation-text {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                padding: 27px 0 0 0;
                margin: 0;

                &__link {
                    font-family: 'font_bold';
                    color: #2683ff;
                }
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
            
            &:hover .close-cross-svg-path {
                fill: #2683FF;
            }
        }
    }
    
    @media screen and (max-width: 720px) {
        .project-creation-success-popup {
        
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
