// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="permissions">
        <BackIcon class="permissions__back-icon" @click="onBackClick"/>
        <h1 class="permissions__title">Permissions</h1>
        <p class="permissions__sub-title">
            Assign permissions to this Access Grant.
        </p>
        <div class="permissions__content">
            <div class="permissions__content__left">
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="read" name="read" v-model="isRead" :checked="isRead">
                    <label class="permissions__content__left__item__label" for="read">Read</label>
                </div>
                <div class="permissions__content__left__item">
                    <input type="checkbox" id="write" name="write" v-model="isWrite" :checked="isWrite">
                    <label class="permissions__content__left__item__label" for="write">Write</label>
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
                <div class="permissions__content__right__buckets">
                    <p class="permissions__content__right__buckets__label">Buckets</p>
                    <BucketsSelection />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';

@Component({
    components: {
        BackIcon,
        BucketsSelection,
    },
})
export default class PermissionsStep extends Vue {
    private key: string = '';
    private restrictedKey: string = '';
    private worker: Worker;

    public isRead: boolean = true;
    public isWrite: boolean = true;
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
        this.worker = new Worker('/static/static/wasm/webWorker.js');
        this.worker.onmessage = (event: MessageEvent) => {
            const data = event.data;
            if (data.error) {
                this.$notify.error(data.error);

                return;
            }

            this.$notify.success('Permissions were set successfully');

            this.restrictedKey = data.value;
        };
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }

    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
    }
}
</script>

<style scoped lang="scss">
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

        &__back-icon {
            position: absolute;
            top: 40px;
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
            margin: 0 0 50px 0;
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

                &__buckets {
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
            }
        }
    }
</style>
