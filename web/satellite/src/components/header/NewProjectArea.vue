// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-container">
        <VInfo
            v-if="isMockButtonShown"
            text="Please add a payment method"
        >
            <div class="new-project-button-mock">
                <h1 class="new-project-button-mock__label">+ Create Project</h1>
            </div>
        </VInfo>
        <div
            v-if="isButtonShown"
            class="new-project-button-container"
            @click="toggleSelection"
            id="newProjectButton"
        >
            <h1 class="new-project-button-container__label">+ Create Project</h1>
        </div>
        <NewProjectPopup v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfo from '@/components/common/VInfo.vue';
import NewProjectPopup from '@/components/project/NewProjectPopup.vue';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { ProjectOwning } from '@/utils/projectOwning';

@Component({
    components: {
        VInfo,
        NewProjectPopup,
    },
})
export default class NewProjectArea extends Vue {
    // TODO: temporary solution. Remove when user will be able to create more then one project
    /**
     * Life cycle hook after initial render.
     * Toggles new project button visibility depending on user having his own project or payment method.
     */
    public mounted(): void {
        if (ProjectOwning.userHasOwnProject() || !this.$store.getters.isBonusCouponApplied) {
            this.$store.dispatch(APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON);

            return;
        }

        this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
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
     * Indicates if new project creation button is shown.
     */
    public get isButtonShown(): boolean {
        return this.$store.state.appStateModule.appState.isCreateProjectButtonShown;
    }

    /**
     * Indicates if new project creation mock button is shown.
     */
    public get isMockButtonShown(): boolean {
        return !ProjectOwning.userHasOwnProject() && !this.$store.getters.isBonusCouponApplied;
    }
}
</script>

<style scoped lang="scss">
    .new-project-container {
        background-color: #fff;
        position: relative;
    }

    .new-project-button-container {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 156px;
        height: 40px;
        border-radius: 6px;
        border: 1px solid #afb7c1;
        background-color: white;
        cursor: pointer;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 15px;
            line-height: 22px;
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

    .new-project-button-mock {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 156px;
        height: 40px;
        border-radius: 6px;
        background-color: #dadde5;
        border: 1px solid #dadde5;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 15px;
            line-height: 22px;
            color: #acb0bc;
        }
    }

    /deep/ .info__message-box {
        background-image: url('../../../static/images/header/info.png');
        background-repeat: no-repeat;
        height: auto;
        width: auto;
        top: 41px;
        left: 157px;
        padding: 30px 20px 25px 20px;
        white-space: nowrap;

        &__text {
            text-align: left;
            font-size: 13px;
            line-height: 17px;
        }
    }
</style>
