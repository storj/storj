// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="access-grant__modal-container__header-container">
            <h2 class="access-grant__modal-container__header-container__title">Create Access</h2>
            <div
                class="access-grant__modal-container__header-container__close-cross-container" @click="onCloseClick"
            >
                <CloseCrossIcon />
            </div>
        </div>
        <div class="access-grant__modal-container__body-container">
            <TypesIcon class="access-grant__modal-container__body-container__type-icon" />
            <div class="access-grant__modal-container__body-container__type">
                <p>Type</p>
                <div class="access-grant__modal-container__body-container__type__type-container">
                    <input
                        id="access-grant-check"
                        v-model="checkedEvent"
                        value="access"
                        type="radio"
                        name="type"
                        @input="typeInput"
                        :checked="checkedType === 'access'"
                    >
                    <label for="access-grant-check">
                        Access Grant
                    </label>
                    <img
                        class="tooltip-icon"
                        src="../../../../static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('access','over')"
                        @mouseleave="toggleTooltipHover('access','leave')"
                    >
                </div>
                <div class="access-grant__modal-container__body-container__type__type-container">
                    <input
                        id="s3-check"
                        v-model="checkedEvent"
                        value="s3"
                        type="radio"
                        name="type"
                        :checked="checkedType === 's3'"
                        @input="typeInput"
                    >
                    <label for="s3-check">S3 Credentials</label>
                    <img
                        class="tooltip-icon"
                        src="../../../../static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('s3','over')"
                        @mouseleave="toggleTooltipHover('s3','leave')"
                    >
                </div>
                <div class="access-grant__modal-container__body-container__type__type-container">
                    <input
                        id="api-check"
                        v-model="checkedEvent"
                        value="api"
                        type="radio"
                        name="type"
                        :checked="checkedType === 'api'"
                        @input="typeInput"
                        @click="checkedType = 'api'"
                    >
                    <label for="api-check">API Access</label>
                    <img
                        class="tooltip-icon"
                        src="../../../../static/images/accessGrants/create-access_information.png"
                        alt="tooltip icon"
                        @mouseover="toggleTooltipHover('api','over')"
                        @mouseleave="toggleTooltipHover('api','leave')"
                    >
                </div>
            </div>
            <NameIcon class="access-grant__modal-container__body-container__name-icon" />
            <div class="access-grant__modal-container__body-container__name">
                <p>Name</p>
                <input
                    v-model="accessName"
                    type="text"
                    placeholder="Input Access Name" class="access-grant__modal-container__body-container__name__input"
                >
            </div>
            <PermissionsIcon class="access-grant__modal-container__body-container__permissions-icon" />
            <div class="access-grant__modal-container__body-container__permissions">
                <p>Permissions</p>
                <div>
                    <input
                        id="permissions__all-check"
                        type="checkbox"
                        :checked="allPermissionsClicked"
                        @click="toggleAllPermission('all')"
                    >
                    <label for="permissions__all-check">All</label>
                    <Chevron :class="`permissions-chevron-${showAllPermissions.position}`" @click="togglePermissions" />
                </div>
                
                <div v-if="showAllPermissions.show === true">
                    <div v-for="(item, key) in permissionsList" :key="key">
                        <input
                            :id="`permissions__${item}-check`"
                            v-model="selectedPermissions"
                            :value="item"
                            type="checkbox"
                            :checked="checkedPermissions.item"
                            @click="toggleAllPermission(item)"
                        >
                        <label :for="`permissions__${item}-check`">{{ item }}</label>
                    </div>
                </div>
            </div>
            <BucketsIcon class="access-grant__modal-container__body-container__buckets-icon" />
            <div class="access-grant__modal-container__body-container__buckets">
                <p>Buckets</p>
                <div>
                    <BucketsSelection
                        class="access-bucket-container"
                        :show-scrollbar="true"
                    />
                </div>
                <div class="access-grant__modal-container__body-container__buckets__bucket-bullets">
                    <div
                        v-for="(name, index) in selectedBucketNames"
                        :key="index"
                        class="access-grant__modal-container__body-container__buckets__bucket-bullets__container"
                    >
                        <BucketNameBullet :name="name" />
                    </div>
                </div>
            </div>
            <DateIcon class="access-grant__modal-container__body-container__date-icon" />
            <div class="access-grant__modal-container__body-container__duration">
                <p>Duration</p>
                <div v-if="addDateSelected">
                    <DurationSelection
                        container-style="access-date-container"
                        text-style="access-date-text"
                        picker-style="__access-date-container"
                    />
                </div>
                <div
                    v-else
                    class="access-grant__modal-container__body-container__duration__text"
                    @click="addDateSelected = true"
                >
                    Add Date (optional)
                </div>
            </div>

        <!-- for future use when notes feature is implemented -->
        <!-- <NotesIcon class="access-grant__modal-container__body-container__notes-icon"/>
                    <div class="access-grant__modal-container__body-container__notes">
                        <p>Notes</p>
                        <div>--Notes Section Here--</div>
                    </div> -->
        </div>
        <div class="access-grant__modal-container__footer-container">
            <a href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key/" target="_blank" rel="noopener noreferrer">
                <v-button
                    label="Learn More"
                    width="150px"
                    height="50px"
                    is-transparent="true"
                    font-size="16px"
                    class="access-grant__modal-container__footer-container__learn-more-button"
                />
            </a>
            <!-- Remove before committing -->
            {{ checkedType }}
            <v-button
                :label="checkedType === 'api' ? 'Create Keys  ⟶' : 'Encrypt My Access  ⟶'"
                font-size="16px"
                width="auto"
                height="50px"
                class="access-grant__modal-container__footer-container__encrypt-button"
                :on-press="checkedType === 'api' ? createAccessGrant : encryptClickAction"
                :is-disabled="selectedPermissions.length === 0 || accessName === ''"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import DateIcon from '@/../static/images/accessGrants/create-access_date.svg';
import VButton from '@/components/common/VButton.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import TypesIcon from '@/../static/images/accessGrants/create-access_type.svg';
import NameIcon from '@/../static/images/accessGrants/create-access_name.svg';
import PermissionsIcon from '@/../static/images/accessGrants/create-access_permissions.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';
import BucketsIcon from '@/../static/images/accessGrants/create-access_buckets.svg';
import BucketNameBullet from "@/components/accessGrants/permissions/BucketNameBullet.vue";
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant } from '@/types/accessGrants';


// @vue/component
@Component({
    components: {
        Chevron,
        CloseCrossIcon,
        TypesIcon,
        NameIcon,
        PermissionsIcon,
        BucketsSelection,
        BucketsIcon,
        BucketNameBullet,
        DateIcon,
        DurationSelection,
        VButton
    },
})


export default class CreateFormModal extends Vue {
    @Prop({ default: '' })
    private checkedType: string;

    public showAllPermissions = {show: false, position: "up"};
    private readonly toggleToolTip: any[];
    private accessName = '';
    private selectedPermissions : string[] = [];
    private allPermissionsClicked = false;
    private permissionsList = ["Read","Write","List","Delete"];
    private checkedPermissions = {Read: false, Write: false, List: false, Delete: false};
    private accessGrantList = this.accessGrantsList;

    public mounted(): void {
        this.showAllPermissions = {show: false, position: "up"};
    }

    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.$emit('close-modal');
    }
    
    public typeInput(e): void {
        console.log(e, 'BOOM');
        this.$emit('input', e);
    }

    /**
     * Retrieves selected buckets for bucket bullets.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }

    public createAccessGrant(): void {
        const payloadObject  = {
            'type': 'SetPermission',
            'buckets': this.selectedBucketNames,
            'isDownload': this.selectedPermissions.includes('Read'),
            'isUpload': this.selectedPermissions.includes('Write'),
            'isList': this.selectedPermissions.includes('List'),
            'isDelete': this.selectedPermissions.includes('Delete'),
        }

        this.$emit('createGrant', payloadObject)

    }

    /**
     * Toggles permissions list visibility.
     */
    public togglePermissions(): void {
        this.showAllPermissions.show = !this.showAllPermissions.show;
        this.showAllPermissions.position = this.showAllPermissions.show ? 'up' : 'down';
    }

    public toggleTooltipHover(type: string, action: string): void {
        this.$emit('toggleToolTip', type, action)
    }

    public get accessGrantsList(): AccessGrant[] {
        return this.$store.state.accessGrantsModule.page.accessGrants;
    }

    public encryptClickAction(): void {
        let mappedList = this.accessGrantList.map((key) => (key.name))
        if (mappedList.includes(this.accessName)) {
            this.$notify.error(`validation: An API Key with this name already exists in this project, please use a different name`);
            return
        } else if (this.checkedType !== "api") {
            // emit event here
            this.$emit('encrypt');
            // this.accessGrantStep = 'encrypt';
        }
    }

    public toggleAllPermission(type): void {
        if (type === 'all' && !this.allPermissionsClicked) {
            this.allPermissionsClicked = true;
            this.selectedPermissions = this.permissionsList;
            this.checkedPermissions = { Read: true, Write: true, List: true, Delete: true }
            return
        } else if(type === 'all' && this.allPermissionsClicked) {
            this.allPermissionsClicked = false;
            this.selectedPermissions = [];
            this.checkedPermissions = { Read: false, Write: false, List: false, Delete: false };
            return
        } else if(this.checkedPermissions[type]) {
            this.checkedPermissions[type] = false;
            this.allPermissionsClicked = false;
            return;
        } else {
            this.checkedPermissions[type] = true;
            if(this.checkedPermissions.Read && this.checkedPermissions.Write && this.checkedPermissions.List && this.checkedPermissions.Delete) {
                this.allPermissionsClicked = true;
            }
        }
    }
}
</script>

<style scoped lang="scss">

    @mixin chevron {
        padding-left: 4px;
        transition: transform 0.3s;
    }

    .permissions-chevron-up {
        @include chevron;
        transform: rotate(-90deg);
    }

    .permissions-chevron-down {
        @include chevron;
    }
    .access-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: flex-start;
        justify-content: center;

        & > * {
            font-family: sans-serif;
        }

        &__modal-container {
            background: #fff;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            position: relative;
            padding: 25px 40px;
            margin-top: 40px;
            width: 410px;
            height: auto;

            &__header-container {
                text-align: left;
                display: grid;
                grid-template-columns: 2fr 1fr;
                width: 100%;
                padding-top: 10px;

                &__title {
                    grid-column: 1;
                }

                &__title-complete {
                    grid-column: 1;
                    margin-top: 10px;
                }

                &__close-cross-container {
                    grid-column: 2;
                    margin: auto 0 auto auto;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    right: 30px;
                    top: 30px;
                    height: 24px;
                    width: 24px;
                    cursor: pointer;
                }

                &__close-cross-container:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }

            &__body-container {
                display: grid;
                grid-template-columns: 1fr 6fr;
                grid-template-rows: auto auto auto auto auto auto;
                grid-row-gap: 24px;
                width: 100%;
                padding-top: 10px;
                margin-top: 24px;

                &__type-icon {
                    grid-column: 1;
                    grid-row: 1;
                }


                &__type {
                    grid-column: 2;
                    grid-row: 1;
                    display: flex;
                    flex-direction: column;

                    &__type-container {
                        display: flex;
                        flex-direction: row;
                        align-items: center;
                        margin-bottom: 10px;
                    }
                }

                &__encrypt {
                    width: 100%;
                    display: flex;
                    flex-flow: column;
                    align-items: center;
                    justify-content: center;
                    margin: 15px 0;

                    &__item {
                        display: flex;
                        align-items: center;
                        justify-content: space-between;
                        width: 100%;
                        height: 40px;
                        box-sizing: border-box;

                        &__left-area {
                            display: flex;
                            align-items: center;
                            justify-content: flex-start;
                        }

                        &__icon {
                            margin-right: 8px;

                            &.selected {

                                ::v-deep circle {
                                    fill: #e6edf7 !important;
                                }

                                ::v-deep path {
                                    fill: #003dc1 !important;
                                }
                            }
                        }

                        &__text {
                            display: flex;
                            flex-direction: column;
                            justify-content: space-between;
                            align-items: flex-start;
                            font-family: 'font_regular', sans-serif;
                            font-size: 12px;

                            h3 {
                                margin: 0 0 8px;
                                font-family: 'font_bold', sans-serif;
                                font-size: 14px;
                            }

                            p {
                                padding: 0;
                            }
                        }

                        &__radio {
                            display: flex;
                            align-items: center;
                            justify-content: center;
                            width: 10px;
                            height: 10px;
                        }
                    }

                    &__divider {
                        width: 100%;
                        height: 1px;
                        background: #ebeef1;
                        margin: 16px 0;

                        &.in-middle {
                            order: 4;
                        }
                    }
                }

                &__created {
                    width: 100%;
                    text-align: left;
                    display: grid;
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    margin-top: 15px;
                    row-gap: 4ch;
                    padding-top: 10px;

                    p {
                        font-style: normal;
                        font-weight: 400;
                        font-size: 14px;
                        line-height: 20px;
                        overflow-wrap: break-word;
                        text-align: left;
                    }
                }

                &__name-icon {
                    grid-column: 1;
                    grid-row: 2;
                }

                &__name {
                    grid-column: 2;
                    grid-row: 2;
                    display: flex;
                    flex-direction: column;
                    max-width: 238px;

                    &__input {
                        background: #fff;
                        border: 1px solid #c8d3de;
                        box-sizing: border-box;
                        border-radius: 6px;
                        height: 40px;
                        font-size: 17px;
                        padding: 10px;
                    }

                    &__input:focus {
                        border-color: #2683ff;
                    }
                }

                &__input:focus {
                    border-color: #2683ff;
                }

                &__permissions-icon {
                    grid-column: 1;
                    grid-row: 3;
                }

                &__permissions {
                    grid-column: 2;
                    grid-row: 3;
                    display: flex;
                    flex-direction: column;
                }

                &__buckets-icon {
                    grid-column: 1;
                    grid-row: 4;
                }

                &__buckets {
                    grid-column: 2;
                    grid-row: 4;
                    display: flex;
                    flex-direction: column;

                    &__bucket-bullets {
                        display: flex;
                        align-items: center;
                        max-width: 100%;
                        flex-wrap: wrap;

                        &__container {
                            display: flex;
                            margin-top: 5px;
                        }
                    }
                }

                &__date-icon {
                    grid-column: 1;
                    grid-row: 5;
                }

                &__duration {
                    grid-column: 2;
                    grid-row: 5;
                    display: flex;
                    flex-direction: column;

                    &__text {
                        color: #929fb1;
                        text-decoration: underline;
                        font-family: sans-serif;
                        cursor: pointer;
                    }
                }

                &__notes-icon {
                    grid-column: 1;
                    grid-row: 6;
                }

                &__notes {
                    grid-column: 2;
                    grid-row: 6;
                    display: flex;
                    flex-direction: column;
                }
            }

            &__footer-container {
                display: flex;
                width: 100%;
                justify-content: flex-start;
                margin-top: 16px;

                & ::v-deep .container:first-of-type {
                    margin-right: 8px;
                }

                &__learn-more-button {
                    padding: 0 15px;
                }

                &__copy-button {
                    width: 49% !important;
                    margin-right: 10px;
                }

                &__download-button {
                    width: 49% !important;
                }

                &__encrypt-button {
                    padding: 0 15px;
                }

                .in-middle {
                    order: 3;
                }
            }
        }
    }

    @media screen and (max-width: 500px) {

        .access-grant__modal-container {
            width: auto;
            max-width: 80vw;
            padding: 30px 24px;

            &__body-container {
                grid-template-columns: 1.2fr 6fr;
            }
        }
    }
</style>




