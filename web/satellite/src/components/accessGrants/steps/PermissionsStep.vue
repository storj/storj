// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="permissions" :class="{ 'border-radius': isOnboardingTour }">
        <BackIcon class="permissions__back-icon" @click="onBackClick" />
        <h1 class="permissions__title">Access Permissions</h1>
        <p class="permissions__sub-title">
            Assign permissions to this Access Grant.
        </p>
        <div class="permissions__content">
            <div class="permissions__content__left">
                <div class="permissions__content__left__item">
                    <input id="download" v-model="isDownload" type="checkbox" name="download" :checked="isDownload">
                    <label class="permissions__content__left__item__label" for="download">Download</label>
                </div>
                <div class="permissions__content__left__item">
                    <input id="upload" v-model="isUpload" type="checkbox" name="upload" :checked="isUpload">
                    <label class="permissions__content__left__item__label" for="upload">Upload</label>
                </div>
                <div class="permissions__content__left__item">
                    <input id="list" v-model="isList" type="checkbox" name="list" :checked="isList">
                    <label class="permissions__content__left__item__label" for="list">List</label>
                </div>
                <div class="permissions__content__left__item">
                    <input id="delete" v-model="isDelete" type="checkbox" name="delete" :checked="isDelete">
                    <label class="permissions__content__left__item__label" for="delete">Delete</label>
                </div>
            </div>
            <div class="permissions__content__right">
                <div class="permissions__content__right__duration-select">
                    <p class="permissions__content__right__duration-select__label">Duration</p>
                    <DurationSelection />
                </div>
                <div class="permissions__content__right__buckets-select">
                    <p class="permissions__content__right__buckets-select__label">Buckets</p>
                    <VLoader v-if="areBucketNamesFetching" width="50px" height="50px" />
                    <BucketsSelection v-else />
                </div>
                <div class="permissions__content__right__bucket-bullets">
                    <div
                        v-for="(name, index) in selectedBucketNames"
                        :key="index"
                        class="permissions__content__right__bucket-bullets__container"
                    >
                        <BucketNameBullet :name="name" />
                    </div>
                </div>
            </div>
        </div>
        <VButton
            class="permissions__button"
            label="Continue in Browser"
            width="100%"
            height="48px"
            :on-press="onContinueInBrowserClick"
            :is-disabled="isLoading || !isAccessGrantsWebWorkerReady || areBucketNamesFetching"
        />
        <p
            class="permissions__cli-link"
            :class="{ disabled: !isAccessGrantsWebWorkerReady || isLoading || areBucketNamesFetching }"
            @click.stop="onContinueInCLIClick"
        >
            Continue in CLI
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { DurationPermission } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';

import BucketNameBullet from '@/components/accessGrants/permissions/BucketNameBullet.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

// @vue/component
@Component({
    components: {
        BackIcon,
        BucketsSelection,
        BucketNameBullet,
        DurationSelection,
        VButton,
        VLoader,
    },
})
export default class PermissionsStep extends Vue {
    private key = '';
    private restrictedKey = '';
    private worker: Worker;

    public isLoading = true;
    public isDownload = true;
    public isUpload = true;
    public isList = true;
    public isDelete = true;
    public areBucketNamesFetching = true;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     * Initializes web worker's onmessage functionality.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key) {
            this.onBackClick();

            return;
        }

        this.key = this.$route.params.key;

        this.setWorker();

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);

            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }

        this.isLoading = false;
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Holds on continue in CLI button click logic.
     */
    public async onContinueInCLIClick(): Promise<void> {
        if (this.isLoading || !this.isAccessGrantsWebWorkerReady) return;

        this.isLoading = true;

        try {
            await this.setPermissions();
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        this.isLoading = false;

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CLIStep)).path);
        await this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CLIStep)).name,
            params: {
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Holds on continue in browser button click logic.
     */
    public async onContinueInBrowserClick(): Promise<void> {
        if (this.isLoading || !this.isAccessGrantsWebWorkerReady) return;

        this.isLoading = true;

        try {
            await this.setPermissions();
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        this.isLoading = false;

        if (this.accessGrantsAmount > 1) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).path);
            await this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).name,
                params: {
                    key: this.key,
                    restrictedKey: this.restrictedKey,
                },
            });

            return;
        }

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).path);
        await this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).name,
            params: {
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Indicates if access grants web worker ready to use.
     */
    public get isAccessGrantsWebWorkerReady(): boolean {
        return this.$store.state.accessGrantsModule.isAccessGrantsWebWorkerReady;
    }

    /**
     * Returns selected bucket names.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Sets chosen permissions for API Key.
     */
    private async setPermissions(): Promise<void> {
        let permissionsMsg = {
            'type': 'SetPermission',
            'buckets': this.selectedBucketNames,
            'apiKey': this.key,
            'isDownload': this.isDownload,
            'isUpload': this.isUpload,
            'isList': this.isList,
            'isDelete': this.isDelete,
        };

        if (this.notBeforePermission) permissionsMsg = Object.assign(permissionsMsg, { 'notBefore': this.notBeforePermission.toISOString() });
        if (this.notAfterPermission) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': this.notAfterPermission.toISOString() });

        await this.worker.postMessage(permissionsMsg);

        const keyEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (keyEvent.data.error) {
            throw new Error(keyEvent.data.error);
        }

        this.restrictedKey = keyEvent.data.value;
        await this.$notify.success('Permissions were set successfully');

        await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, new DurationPermission());
    }

    /**
     * Returns amount of access grants from store.
     */
    private get accessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.page.accessGrants.length;
    }

    /**
     * Returns not before date permission from store.
     */
    private get notBeforePermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotBefore;
    }

    /**
     * Returns not after date permission from store.
     */
    private get notAfterPermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotAfter;
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

    .permissions {
        height: calc(100% - 60px);
        width: calc(100% - 130px);
        padding: 30px 65px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 10px;
        }

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 70px;
        }

        &__content {
            display: flex;
            width: 100%;

            &__left {
                display: flex;
                flex-direction: column;
                align-items: flex-start;

                &__item {
                    display: flex;
                    align-items: center;
                    flex-wrap: nowrap;
                    margin-bottom: 15px;

                    &__label {
                        margin: 0 0 0 10px;
                    }
                }
            }

            &__right {
                width: 100%;
                margin-left: 100px;

                &__buckets-select,
                &__duration-select {
                    display: flex;
                    align-items: center;
                    width: 100%;

                    &__label {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                        margin: 0;
                    }
                }

                &__duration-select {
                    margin-bottom: 40px;
                }

                &__bucket-bullets {
                    display: flex;
                    align-items: center;
                    flex-wrap: wrap;
                    margin: 15px 0 0 85px;
                    max-height: 100px;
                    max-width: 235px;
                    overflow-x: hidden;
                    overflow-y: scroll;
                }
            }
        }

        &__button {
            margin-top: 60px;
        }

        &__cli-link {
            font-family: 'font_medium', sans-serif;
            cursor: pointer;
            font-weight: 600;
            font-size: 16px;
            line-height: 23px;
            color: #0068dc;
            margin-top: 20px;
        }
    }

    .border-radius {
        border-radius: 6px;
    }

    .disabled {
        pointer-events: none;
        color: rgb(0 0 0 / 40%);
    }
</style>
