// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-name-creation">
        <v-loader v-if="isLoading" width="100px" height="100px" />
        <template v-else>
            <bucket-icon />
            <div class="bucket-name-creation__functional">
                <div class="bucket-name-creation__functional__header">
                    <p class="bucket-name-creation__functional__header__title" aria-roledescription="title">
                        Create a bucket
                    </p>
                    <p class="bucket-name-creation__functional__header__info">
                        Buckets are used to store your files. It’s recommended that every bucket should have it’s own encryption passphrase.
                    </p>
                </div>
                <headered-input
                    label="Bucket Name"
                    placeholder="Enter a passphrase here..."
                    :error="nameError"
                    role-description="name"
                    :init-value="name"
                    additional-label="Lowercase alphanumeric characters only (no spaces)"
                    @setData="onChangeName"
                />
            </div>
            <div class="bucket-name-creation__buttons">
                <v-button
                    label="Cancel"
                    height="48px"
                    width="45%"
                    :on-press="onCancelClick"
                    :is-white="true"
                />
                <v-button
                    label="Continue"
                    height="48px"
                    width="45%"
                    :on-press="onNextButtonClick"
                    :is-disabled="!name || nameError !== ''"
                />
            </div>
        </template>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { Validator } from "@/utils/validation";
import { LocalData } from "@/utils/localData";
import { BUCKET_ACTIONS } from "@/store/modules/buckets";

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from "@/components/common/VLoader.vue";

import BucketIcon from "@/../static/images/objects/bucketCreation.svg";

// @vue/component
@Component({
    components: {
        HeaderedInput,
        VButton,
        BucketIcon,
        VLoader,
    },
})
export default class BucketCreationNameStep extends Vue {
    public name = '';
    public nameError = '';
    public isLoading = true;

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES)
            this.name = this.allBucketNames.length > 0 ? '' : 'demo-bucket';
        } catch (e) {
            await this.$notify.error(e.message)
        } finally {
            this.isLoading = false;
        }
    }

    /**
     * Sets bucket name from input.
     */
    public onChangeName(value: string): void {
        if (this.nameError) this.nameError = '';
        this.name = value;
    }

    /**
     * Sets bucket name from input.
     */
    public onNextButtonClick(): void {
        if (!this.isBucketNameValid(this.name)) return;

        if (this.allBucketNames.includes(this.name)) {
            this.$notify.error("Bucket with this name already exists");
            return;
        }

        this.$emit('setName', this.name);
    }

    /**
     * Redirects to buckets list view.
     */
    public onCancelClick(): void {
        LocalData.setDemoBucketCreatedStatus();
        this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
    }

    /**
     * Returns validation status of a bucket name.
     */
    private isBucketNameValid(name: string): boolean {
        switch (true) {
        case name.length < 3 || name.length > 63:
            this.nameError = 'Name must be not less than 3 and not more than 63 characters length';

            return false;
        case !Validator.bucketName(name):
            this.nameError = 'Name must contain only lowercase latin characters, numbers, a hyphen or a period';

            return false;

        default:
            return true;
        }
    }

    private get allBucketNames(): string[] {
        return this.$store.state.bucketUsageModule.allBucketNames;
    }
}
</script>

<style scoped lang="scss">
.bucket-name-creation {
    font-family: 'font_regular', sans-serif;
    padding: 60px 65px 50px;
    max-width: 500px;
    background: #fcfcfc;
    box-shadow: 0 0 32px rgb(0 0 0 / 4%);
    border-radius: 20px;
    margin: 30px auto 0;
    display: flex;
    flex-direction: column;
    align-items: center;

    &__functional {
        padding: 20px 0;

        &__header {
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;
            padding: 25px 0;
            text-align: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 26px;
                line-height: 31px;
                color: #131621;
                margin-bottom: 15px;
            }

            &__info {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }
        }
    }

    &__buttons {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 30px;

        &__back {
            margin-right: 24px;
        }
    }
}

::v-deep .label-container__main__label {
    font-size: 14px;
}

::v-deep .label-container__main {
    width: 100%;
    justify-content: space-between;
}
</style>
