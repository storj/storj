// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-grant">
        <div class="create-grant__container">
            <ProgressBar v-if="!isProgressBarHidden" />
            <router-view />
            <div class="create-grant__container__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';

import ProgressBar from '@/components/accessGrants/ProgressBar.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        ProgressBar,
        CloseCrossIcon,
    },
})
export default class CreateAccessGrant extends Vue {

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.analytics.pageVisit(RouteConfig.AccessGrants.path);
        this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Indicates if progress bar is hidden.
     */
    public get isProgressBarHidden(): boolean {
        return this.$route.name === RouteConfig.CLIStep.name;
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        margin: 0;
        width: 0;
    }

    .create-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;

        &__container {
            background: #f5f6fa;
            border-radius: 6px;
            display: flex;
            align-items: flex-start;
            position: relative;

            &__close-cross-container {
                display: flex;
                justify-content: center;
                align-items: center;
                position: absolute;
                right: 30px;
                top: 30px;
                height: 24px;
                width: 24px;
                cursor: pointer;

                &:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }
        }
    }

    @media screen and (max-height: 800px) {

        .create-grant {
            padding: 50px 0 20px;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 750px) {

        .create-grant {
            padding: 100px 0 20px;
        }
    }

    @media screen and (max-height: 700px) {

        .create-grant {
            padding: 150px 0 20px;
        }
    }

    @media screen and (max-height: 650px) {

        .create-grant {
            padding: 200px 0 20px;
        }
    }

    @media screen and (max-height: 600px) {

        .create-grant {
            padding: 250px 0 20px;
        }
    }

    @media screen and (max-height: 550px) {

        .create-grant {
            padding: 300px 0 20px;
        }
    }

    @media screen and (max-height: 500px) {

        .create-grant {
            padding: 350px 0 20px;
        }
    }
</style>
