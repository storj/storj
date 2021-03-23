// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buckets-view">
        <div class="buckets-view__title-area">
            <h1 class="buckets-view__title-area__title">Buckets</h1>
            <div class="buckets-view__title-area__button" @click="showCreateBucketPopup">
                <BucketIcon/>
                <p class="buckets-view__title-area__button__label">New Bucket</p>
            </div>
        </div>
        <div class="buckets-view__loader" v-if="isLoading"/>
        <p class="buckets-view__no-buckets" v-if="!(isLoading || buckets.length)">No Buckets</p>
        <div class="buckets-view__list" v-if="!isLoading && buckets.length">
            <div class="buckets-view__list__sorting-header">
                <p class="buckets-view__list__sorting-header__name">Name</p>
                <p class="buckets-view__list__sorting-header__date">Date Added</p>
                <p class="buckets-view__list__sorting-header__empty"/>
            </div>
            <div class="buckets-view__list__item" v-for="(bucket, key) in buckets" :key="key" @click.stop="openBucket">
                <BucketItem
                    :item-data="bucket"
                    :show-delete-bucket-popup="showDeleteBucketPopup"
                    :dropdown-key="key"
                    :open-dropdown="openDropdown"
                    :is-dropdown-open="activeDropdown === key"
                />
            </div>
        </div>
        <ObjectsPopup
            v-if="isCreatePopupVisible"
            @setName="setCreateBucketName"
            @close="hideCreateBucketPopup"
            :on-click="onCreateBucketClick"
            title="Create Bucket"
            sub-title="Buckets are simply containers that store objects and their metadata within a project."
            button-label="Create Bucket"
            :error-message="errorMessage"
            :is-loading="isRequestProcessing"
        />
        <ObjectsPopup
            v-if="isDeletePopupVisible"
            @setName="setDeleteBucketName"
            @close="hideDeleteBucketPopup"
            :on-click="onDeleteBucketClick"
            title="Are you sure?"
            sub-title="Deleting this bucket will delete all metadata related to this bucket."
            button-label="Confirm Delete Bucket"
            :default-input-value="deleteBucketName"
            :error-message="errorMessage"
            :is-loading="isRequestProcessing"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketItem from '@/components/objects/BucketItem.vue';
import ObjectsPopup from '@/components/objects/ObjectsPopup.vue';

import BucketIcon from '@/../static/images/objects/bucket.svg';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AccessGrant, GatewayCredentials } from '@/types/accessGrants';
import { MetaUtils } from '@/utils/meta';
import { Bucket } from '@aws-sdk/client-s3';

@Component({
    components: {
        BucketIcon,
        ObjectsPopup,
        BucketItem,
    },
})
export default class BucketsView extends Vue {
    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';
    private worker: Worker;
    private grantWithPermissions: string = '';
    private accessGrant: string = '';
    private createBucketName: string = '';
    private deleteBucketName: string = '';

    public isLoading: boolean = true;
    public isCreatePopupVisible: boolean = false;
    public isDeletePopupVisible: boolean = false;
    public isRequestProcessing: boolean = false;
    public errorMessage: string = '';
    public activeDropdown: number = -1;

    /**
     * Lifecycle hook after initial render.
     * Setup gateway credentials.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.state.objectsModule.passphrase) {
            await this.$router.push(RouteConfig.Objects.with(RouteConfig.EnterPassphrase).path);

            return;
        }

        await this.removeTemporaryAccessGrant();

        try {
            const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.FILE_BROWSER_AG_NAME);

            this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
            this.worker.onmessage = (event: MessageEvent) => {
                const data = event.data;
                if (data.error) {
                    throw new Error(data.error);
                }

                this.grantWithPermissions = data.value;
            };

            const now = new Date();
            const inADay = new Date(now.setDate(now.getDate() + 1));

            await this.worker.postMessage({
                'type': 'SetPermission',
                'isDownload': true,
                'isUpload': true,
                'isList': true,
                'isDelete': true,
                'buckets': [],
                'apiKey': cleanAPIKey.secret,
                'notBefore': now.toISOString(),
                'notAfter': inADay.toISOString(),
            });

            // Timeout is used to give some time for web worker to return value.
            setTimeout(() => {
                this.worker.onmessage = (event: MessageEvent) => {
                    const data = event.data;
                    if (data.error) {
                        throw new Error(data.error);
                    }

                    this.accessGrant = data.value;
                };

                const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');
                this.worker.postMessage({
                    'type': 'GenerateAccess',
                    'apiKey': this.grantWithPermissions,
                    'passphrase': this.$route.params.passphrase,
                    'projectID': this.$store.getters.selectedProject.id,
                    'satelliteNodeURL': satelliteNodeURL,
                });

                // Timeout is used to give some time for web worker to return value.
                setTimeout(async () => {
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_ACCESS_GRANT, this.accessGrant);

                    // TODO: use this value until all the satellites will have this URL set.
                    const gatewayURL = 'https://auth.tardigradeshare.io';
                    const gatewayCredentials: GatewayCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: this.accessGrant, optionalURL: gatewayURL});
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
                    await this.$store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
                    await this.$store.dispatch(OBJECTS_ACTIONS.FETCH_BUCKETS);

                    this.isLoading = false;

                    if (!this.buckets.length) this.showCreateBucketPopup();
                }, 1000);
            }, 1000);
        } catch (error) {
            await this.$notify.error(`Failed to setup Objects view. ${error.message}`);

            return;
        }
    }

    /**
     * Lifecycle hook before component destroying.
     * Remove temporary created access grant.
     */
    public async beforeDestroy(): Promise<void> {
        await this.removeTemporaryAccessGrant();
    }

    /**
     * Holds create bucket click logic.
     */
    public async onCreateBucketClick(): Promise<void> {
        if (this.isRequestProcessing) return;

        if (!this.createBucketName) {
            this.errorMessage = 'Bucket name can\'t be empty';
        }

        this.isRequestProcessing = true;

        try {
            await this.$store.dispatch(OBJECTS_ACTIONS.CREATE_BUCKET, this.createBucketName);
            await this.$store.dispatch(OBJECTS_ACTIONS.FETCH_BUCKETS);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isRequestProcessing = false;
        this.createBucketName = '';
        this.hideCreateBucketPopup();
    }

    /**
     * Holds delete bucket click logic.
     */
    public async onDeleteBucketClick(): Promise<void> {
        if (this.isRequestProcessing) return;

        if (!this.deleteBucketName) {
            this.errorMessage = 'Bucket name can\'t be empty';
        }

        this.isRequestProcessing = true;

        try {
            await this.$store.dispatch(OBJECTS_ACTIONS.DELETE_BUCKET, this.deleteBucketName);
            await this.$store.dispatch(OBJECTS_ACTIONS.FETCH_BUCKETS);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isRequestProcessing = false;
        this.deleteBucketName = '';
        this.hideDeleteBucketPopup();
    }

    /**
     * Opens utils dropdown.
     */
    public openDropdown(key: number): void {
        this.activeDropdown = key;
    }

    /**
     * Makes delete bucket popup visible.
     */
    public showDeleteBucketPopup(name: string): void {
        this.deleteBucketName = name;
        this.isDeletePopupVisible = true;
    }

    /**
     * Hides delete bucket popup.
     */
    public hideDeleteBucketPopup(): void {
        this.isDeletePopupVisible = false;
    }

    /**
     * Set delete bucket name form input.
     */
    public setDeleteBucketName(name: string): void {
        this.errorMessage = '';
        this.deleteBucketName = name;
    }

    /**
     * Makes create bucket popup visible.
     */
    public showCreateBucketPopup(): void {
        this.isCreatePopupVisible = true;
    }

    /**
     * Hides create bucket popup.
     */
    public hideCreateBucketPopup(): void {
        this.isCreatePopupVisible = false;
    }

    /**
     * Set create bucket name form input.
     */
    public setCreateBucketName(name: string): void {
        this.errorMessage = '';
        this.createBucketName = name;
    }

    /**
     * Removes temporary created access grant.
     */
    public async removeTemporaryAccessGrant(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public openBucket(): void {
        this.$router.push(RouteConfig.Objects.with(RouteConfig.UploadFile).path);
    }

    /**
     * Returns fetched buckets from store.
     */
    public get buckets(): Bucket[] {
        return this.$store.state.objectsModule.buckets;
    }
}
</script>

<style scoped lang="scss">
    .buckets-view {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        background-color: #f5f6fa;

        &__title-area {
            width: 100%;
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-weight: bold;
                font-size: 18px;
                line-height: 26px;
                color: #232b34;
                margin: 0;
                width: 100%;
                text-align: left;
            }

            &__button {
                width: 154px;
                height: 46px;
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: #0068dc;
                border-radius: 4px;
                cursor: pointer;

                &__label {
                    font-weight: normal;
                    font-size: 12px;
                    line-height: 17px;
                    color: #fff;
                    margin: 0 0 0 5px;
                }

                &:hover {
                    background-color: #0000c2;
                }
            }
        }

        &__loader {
            margin-top: 100px;
            border: 16px solid #f3f3f3;
            border-top: 16px solid #3498db;
            border-radius: 50%;
            width: 120px;
            height: 120px;
            animation: spin 2s linear infinite;
        }

        &__no-buckets {
            width: 100%;
            text-align: center;
            font-size: 30px;
            line-height: 42px;
            margin: 100px 0 0 0;
        }

        &__list {
            margin-top: 40px;
            width: 100%;
            display: flex;
            flex-direction: column;
            overflow: hidden;
            padding-bottom: 100px;

            &__sorting-header {
                display: flex;
                align-items: center;
                padding: 0 20px 5px 20px;
                width: calc(100% - 40px);
                font-weight: bold;
                font-size: 14px;
                line-height: 20px;
                color: #768394;
                border-bottom: 1px solid rgba(169, 181, 193, 0.4);

                &__name {
                    width: calc(70% - 16px);
                    margin: 0;
                }

                &__date {
                    width: 30%;
                    margin: 0;
                }

                &__empty {
                    margin: 0;
                }
            }
        }
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
</style>
