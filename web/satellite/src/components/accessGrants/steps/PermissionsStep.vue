// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="permissions">
        <BackIcon class="permissions__back-icon" @click="onBackClick"/>
        <h1 class="permissions__title">Access Permissions</h1>
        <p class="permissions__sub-title">
            Assign permissions to this Access Grant.
        </p>
        <div class="permissions__content">
            <div class="permissions__content__left">
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="download" name="download" v-model="isDownload" :checked="isDownload">
                    <label class="permissions__content__left__item__label" for="download">Download</label>
                </div>
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="upload" name="upload" v-model="isUpload" :checked="isUpload">
                    <label class="permissions__content__left__item__label" for="upload">Upload</label>
                </div>
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="list" name="list" v-model="isList" :checked="isList">
                    <label class="permissions__content__left__item__label" for="list">List</label>
                </div>
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="delete" name="delete" v-model="isDelete" :checked="isDelete">
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
                    <BucketsSelection />
                </div>
                <div class="permissions__content__right__bucket-bullets">
                    <div
                        class="permissions__content__right__bucket-bullets__container"
                        v-for="(name, index) in storedBucketNames"
                        :key="index"
                    >
                        <BucketNameBullet :name="name"/>
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
            :is-disabled="isLoading"
        />
        <p class="permissions__cli-link" @click.stop="onContinueInCLIClick">
            Continue in CLI
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketNameBullet from '@/components/accessGrants/permissions/BucketNameBullet.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';
import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';

@Component({
    components: {
        BackIcon,
        BucketsSelection,
        BucketNameBullet,
        DurationSelection,
        VButton,
    },
})
export default class PermissionsStep extends Vue {
    private key: string = '';
    private restrictedKey: string = '';
    private worker: Worker;

    public isLoading: boolean = true;
    public isDownload: boolean = true;
    public isUpload: boolean = true;
    public isList: boolean = true;
    public isDelete: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     * Initializes web worker's onmessage functionality.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key) {
            this.onBackClick();
        }

        this.key = this.$route.params.key;
        this.worker = await new Worker('/static/static/wasm/webWorker.js');
        this.worker.onmessage = (event: MessageEvent) => {
            const data = event.data;
            if (data.error) {
                this.$notify.error(data.error);

                return;
            }

            this.restrictedKey = data.value;

            this.$notify.success('Permissions were set successfully');
        };
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
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
        const PREVIOUS_ROUTE_NUMBER: number = -1;

        this.$router.go(PREVIOUS_ROUTE_NUMBER);
    }

    /**
     * Holds on continue in CLI button click logic.
     */
    public onContinueInCLIClick(): void {
        if (this.isLoading) return;

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CLIStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Holds on continue in browser button click logic.
     */
    public onContinueInBrowserClick(): void {
        if (this.isLoading) return;

        this.isLoading = true;

        this.worker.postMessage({
            'type': 'SetPermission',
            'isDownload': this.isDownload,
            'isUpload': this.isUpload,
            'isList': this.isList,
            'isDelete': this.isDelete,
            'buckets': this.selectedBucketNames,
            'apiKey': this.key,
        });

        // Give time for web worker to return value.
        setTimeout(() => {
            this.isLoading = false;

            if (this.accessGrantsAmount > 1) {
                this.$router.push({
                    name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).name,
                    params: {
                        key: this.restrictedKey,
                    },
                });

                return;
            }

            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).name,
                params: {
                    key: this.restrictedKey,
                },
            });
        }, 1000);
    }

    /**
     * Returns stored selected bucket names.
     */
    public get storedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }

    /**
     * Returns selected bucket names from store or all bucket names.
     */
    private get selectedBucketNames(): string[] {
        return this.storedBucketNames.length ? this.storedBucketNames : this.allBucketNames;
    }

    /**
     * Returns amount of access grants from store.
     */
    private get accessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.page.accessGrants.length;
    }

    /**
     * Returns all bucket names.
     */
    private get allBucketNames(): string[] {
        return this.$store.state.bucketUsageModule.allBucketNames;
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
            margin: 0 0 10px 0;
        }

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 70px 0;
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
                    max-height: 200px;
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
        }
    }
</style>
