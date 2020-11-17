// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-grant">
        <div class="create-grant__container">
            <ProgressBar v-if="!isProgressBarHidden"/>
            <router-view/>
            <div class="create-grant__container__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/accessGrants/ProgressBar.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        ProgressBar,
        CloseCrossIcon,
    },
})
export default class CreateAccessGrant extends Vue {
    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Indicates if progress bar is hidden.
     */
    public get isProgressBarHidden(): boolean {
        return this.$route.name === RouteConfig.CLIStep.name || this.$route.name === RouteConfig.UploadStep.name;
    }
}
</script>

<style scoped lang="scss">
    .create-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgba(27, 37, 51, 0.75);
        display: flex;
        align-items: center;
        justify-content: center;

        &__container {
            background: #fff;
            border-radius: 6px;
            display: flex;
            align-items: center;
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
</style>