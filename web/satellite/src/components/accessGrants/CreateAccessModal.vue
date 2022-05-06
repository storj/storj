// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-access">
        
        <div class="create-access__modal-container">
            <div 
                v-if="this.tooltipHover === 'access'" 
                class="access-tooltip"
                @mouseover="toggleTooltipHover('access','over')"
                @mouseleave="toggleTooltipHover('access','leave')"
            >
                <span class="tooltip-text">Keys to upload, delete, and view your project's data.  <a class="tooltip-link" href="https://storj-labs.gitbook.io/dcs/concepts/access/access-grants" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <div 
                v-if="this.tooltipHover === 's3'" 
                class="s3-tooltip"
                @mouseover="toggleTooltipHover('s3','over')"
                @mouseleave="toggleTooltipHover('s3','leave')"
            >
                <span class="tooltip-text">Generates access key, secret key, and endpoint to use in your S3-supporting application.  <a class="tooltip-link" href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <div 
                v-if="this.tooltipHover === 'api'" 
                class="api-tooltip"
                @mouseover="toggleTooltipHover('api','over')"
                @mouseleave="toggleTooltipHover('api','leave')"
            >
                <span class="tooltip-text">Creates access grant to run in the command line.  <a class="tooltip-link" href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token/" target="_blank" rel="noreferrer noopener">Learn More</a></span>
            </div>
            <form>
                <div class="create-access__modal-container__header-container">
                    <h2 class="create-access__modal-container__header-container__title">Create Access</h2>
                    <div
                    class="create-access__modal-container__header-container__close-cross-container" @click="onCloseClick">
                        <CloseCrossIcon />
                    </div>
                </div>
                <div class="create-access__modal-container__body-container">
                    <TypesIcon class="create-access__modal-container__body-container__type-icon"/>
                    <div class="create-access__modal-container__body-container__type">
                        <p>Type</p>
                        <div class="create-access__modal-container__body-container__type__type-container">
                            <input 
                            v-model="checkedType"
                            value="access"
                            type="radio" 
                            id="acess-grant-check"
                            name="type" 
                            :checked="this.checkedType === 'access'"/>
                            <label for="acess-grant-check">
                                Access Grant
                            </label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('access','over')"
                                @mouseleave="toggleTooltipHover('access','leave')"
                            />
                        </div>
                        <div class="create-access__modal-container__body-container__type__type-container">
                            <input 
                            v-model="checkedType"
                            value="s3"
                            type="radio" 
                            id="s3-check"
                            name="type" 
                            :checked="this.checkedType === 's3'"/>
                            <label for="s3-check">S3 Credentials</label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('s3','over')"
                                @mouseleave="toggleTooltipHover('s3','leave')"
                            />
                        </div>
                        <div class="create-access__modal-container__body-container__type__type-container">
                            <input
                            v-model="checkedType"
                            value="api"
                            type="radio" 
                            id="api-check"
                            name="type" 
                            :checked="this.checkedType === 'api'"
                            @click="this.checkedType = 'api'"/>
                            <label for="api-check">API Access</label>
                            <img
                                class="tooltip-icon"
                                src="../../../static/images/accessGrants/create-access_information.png"
                                @mouseover="toggleTooltipHover('api','over')"
                                @mouseleave="toggleTooltipHover('api','leave')"
                            />
                        </div>
                    </div>
                    <NameIcon class="create-access__modal-container__body-container__name-icon"/>
                    <div class="create-access__modal-container__body-container__name">
                        <p>Name</p>
                        <input
                        v-model="accessName"
                        type="text" 
                        placeholder="Input Access Name" class="create-access__modal-container__body-container__name__input"
                        >
                    </div>
                    <PermissionsIcon class="create-access__modal-container__body-container__permissions-icon"/>
                    <div class="create-access__modal-container__body-container__permissions">
                        <p>Permissions</p>
                        <div>
                            <input
                            type="checkbox" 
                            id="permissions__all-check"
                            @click="toggleAllPermission('all')"
                            :checked="allPermissionsClicked"
                            />
                            <label for="permissions__all-check">All</label>
                            <Chevron @click="togglePermissions" :class="`permissions-chevron-${this.showAllPermissions.position}`"/>
                        </div>
                        <div v-if="this.showAllPermissions.show">
                            <div v-for="(item, key) in permissionsList" v-bind:key="key">
                                <input 
                                v-model="selectedPermissions"
                                :value="item"
                                type="checkbox" 
                                :id="`permissions__${item}-check`"
                                :checked="checkedPermissions.item"
                                @click="toggleAllPermission(item)"/>
                                <label :for="`permissions__${item}-check`">{{item}}</label>
                            </div>
                        </div>
                    </div>
                    <BucketsIcon class="create-access__modal-container__body-container__buckets-icon"/>
                    <div class="create-access__modal-container__body-container__buckets">
                        <p>Buckets</p>
                        <div>
                            <BucketsSelection 
                            container-style="access-bucket-container"
                            text-style="access-bucket-text"/>
                        </div>
                        <div class="create-access__modal-container__body-container__buckets__bucket-bullets">
                            <div
                                v-for="(name, index) in selectedBucketNames"
                                :key="index"
                                class="create-access__modal-container__body-container__buckets__bucket-bullets__container"
                            >
                                <BucketNameBullet :name="name" />
                            </div>
                        </div>
                    </div>
                    <DateIcon class="create-access__modal-container__body-container__date-icon"/>
                    <div class="create-access__modal-container__body-container__duration">
                        <p>Duration</p>
                        <div>
                            <DurationSelection
                            container-style="access-date-container"
                            text-style="access-date-text"/>
                        </div>
                    </div>

                    <!-- for future use -->
                    <!-- <NotesIcon class="create-access__modal-container__body-container__notes-icon"/>
                    <div class="create-access__modal-container__body-container__notes">
                        <p>Notes</p>
                        <div>--Notes Section Here--</div>
                    </div> -->

                </div>
                <div class="create-access__modal-container__divider"></div>
                <div class="create-access__modal-container__footer-container">
                    <VButton
                        label="Learn More"
                        width="auto"
                        height="50px"
                        is-transparent="true"
                        font-size="16px"
                        class="create-access__modal-container__footer-container__learn-more-button"
                    />
                    <VButton
                        label="Encrypt My Access  ⟶"
                        font-size="16px"
                        width="auto"
                        height="50px"
                        class="create-access__modal-container__footer-container__encrypt-button"
                    />
                </div>
            </form>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import VButton from '@/components/common/VButton.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import BucketNameBullet from "@/components/accessGrants/permissions/BucketNameBullet.vue";
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import VCheckbox from '@/components/common/VCheckbox.vue';
import TypesIcon from '@/../static/images/accessGrants/create-access_type.svg';
import PermissionsIcon from '@/../static/images/accessGrants/create-access_permissions.svg';
import NameIcon from '@/../static/images/accessGrants/create-access_name.svg';
import BucketsIcon from '@/../static/images/accessGrants/create-access_buckets.svg';
import DateIcon from '@/../static/images/accessGrants/create-access_date.svg';
import NotesIcon from '@/../static/images/accessGrants/create-access_notes.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';
// import InformationIcon from '@/../static/images/accessGrants/create-access_information.png';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from "@/store/modules/buckets";
import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";

import { RouteConfig } from '@/router';

// @vue/component
@Component({
    components: {
        VCheckbox,
        VButton,
        DurationSelection,
        BucketsSelection,
        BucketNameBullet,
        CloseCrossIcon,
        TypesIcon,
        PermissionsIcon,
        NameIcon,
        BucketsIcon,
        DateIcon,
        NotesIcon,
        Chevron,
        // InformationIcon,
    },
})
export default class CreateAccessModal extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    @Prop({default: 'Default'})
    private readonly defaultType: string;
    @Prop({default: () => { return; }})
    private readonly onClose: () => void;

    private checkedType = '';
    private showAllPermissions = {show: false, position: "up"};
    private permissionsList = ["read","write","list","delete"];
    private accessName = '';
    private checkedPermissions = {read: false, write: false, list: false, delete: false};
    private selectedPermissions : string[] = [];
    private allPermissionsClicked = false;
    public areBucketNamesFetching = true;
    public tooltipHover = '';
    public tooltipVisibilityTimer;

    /**
     * Checks which type was selected on mount.
     */
    public async mounted(): Promise<void> {
        this.checkedType = this.defaultType;
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }
    }

    /**
     * Closes modal.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.onClose();
    }

    /**
     * Toggles permissions list visibility.
     */
    public togglePermissions(): void {
        if (this.showAllPermissions.show === false) {
            this.showAllPermissions.show = true;
            this.showAllPermissions.position = "down";
        } else {
            this.showAllPermissions.show = false;
            this.showAllPermissions.position = "up";
        }
    }

    /**
     * Toggles tooltip visibility.
     */
    public toggleTooltipHover(type,action): void {
        if (this.tooltipHover === '' && action === 'over') {
            this.tooltipHover = type;
            return;
        } else if (this.tooltipHover === type && action === 'leave') {
            this.tooltipVisibilityTimer = setTimeout(() => {
                this.tooltipHover = '';
            },750);
            return;
        } else if (this.tooltipHover === type && action === 'over') {
            clearTimeout(this.tooltipVisibilityTimer);
            return;
        } else if(this.tooltipHover !== type) {
            clearTimeout(this.tooltipVisibilityTimer)
            this.tooltipHover = type;
        }
    }

    /**
     * Handles permissions All.
     */
    public toggleAllPermission(permission): void {
        if (permission === 'all' && this.allPermissionsClicked === false) {
            this.allPermissionsClicked = true;
            this.selectedPermissions = this.permissionsList;
            this.checkedPermissions = {read: true, write: true, list: true, delete: true}
            return
        } else if(permission === 'all' && this.allPermissionsClicked === true) {
            this.allPermissionsClicked = false;
            this.selectedPermissions = []
            this.checkedPermissions = {read: false, write: false, list: false, delete: false}
            return
        } else if(this.checkedPermissions[permission] === true) {
            this.checkedPermissions[permission] = false
            this.allPermissionsClicked = false
            return
        } else {
            this.checkedPermissions[permission] = true
            if(this.checkedPermissions.read === true && this.checkedPermissions.write === true && this.checkedPermissions.list === true && this.checkedPermissions.delete === true) {
                this.allPermissionsClicked = true
                return
            }
        }
    }

    /**
     * retrieves selected buckets for bucket bullets.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
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

    @mixin chevron {
        padding-left: 4px;
        transition: transform 0.3s;
    }

    @mixin tooltip-container {
        position: absolute;
        background: #56606D;
        border-radius: 6px;
        width: 253px;
        color: #ffffff;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        padding: 8px;
        z-index: 1;
        transition: 250ms;
    }

    @mixin tooltip-arrow {
        content: "";
        position: absolute;
        bottom: 0;
        width: 0;
        height: 0;
        border: 6px solid transparent;
        border-top-color: #56606d;
        border-bottom: 0;
        margin-left: -20px;
        margin-bottom: -20px;
    }

    p {
      font-weight: bold;
      padding-bottom: 5px;
    }

    label {
        margin-left: 5px;
        padding-right: 10px;
    }

    h2{
        font-weight: 800;
        font-size: 28px;
    }

    form {
        width: 100%;
    }

    .create-access {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgba(27, 37, 51, 0.75);
        display: flex;
        align-items: flex-start;
        justify-content: center;
        & > * {
            font-family: sans-serif;
        }
        
        &__modal-container {
            background: #ffffff;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            position: relative;
            padding: 25px;
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

                    &:hover .close-cross-svg-path {
                        fill: #2683ff;
                    }
                }
            }
            &__body-container {
                display: grid;
                grid-template-columns: 1fr 6fr;
                grid-template-rows: auto auto auto auto auto auto;
                width: 100%;
                padding-top: 10px;
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
                    &__input {
                        background: #FFFFFF;
                        border: 1px solid #C8D3DE;
                        box-sizing: border-box;
                        border-radius: 4px;
                        height: 40px;
                        font-size: 17px;
                        padding: 10px;
                    }
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
                & div {
                    padding-bottom: 10px;
                }
            }
            &__divider {
                height: 1px;
                background-color: #dadfe7;
                margin: 10px auto 0 auto;
                width: 90%;
            }
            &__footer-container {
                display: flex;
                width: 100%;
                justify-content: space-evenly;
                padding-top: 25px;
                &__learn-more-button {
                    padding: 0 15px;
                }
                &__encrypt-button {
                    padding: 0 15px;
                }
            }
        }
    }

    .permissions-chevron-up {
        @include chevron;
        transform: rotate(-90deg);
    }

    .permissions-chevron-down {
        @include chevron;
    }

    .tooltip-icon {
        width: 14px;
        height: 14px;
        cursor: pointer;
    }

    .tooltip-text {
        text-align: center;
        font-weight: 500;
    }

    a {
        color: #FFFFFF;
        text-decoration: underline !important;
        cursor: pointer;
    }

    .access-tooltip {
        top: 52px;
        left: 94px;
        @include tooltip-container;
        &::after {
            left: 50%;
            top: 100%;
            @include tooltip-arrow;
        }
    }

    .s3-tooltip {
        top: 158px;
        left: 103px;
        @include tooltip-container;
        &::after {            
            left: 50%;
            top: -8%;
            transform: rotate(180deg);
            @include tooltip-arrow;
        }
    }

    .api-tooltip {
        top: 186px;
        left: 78px;
        @include tooltip-container;
        &::after {
            left: 50%;
            top: -11%;
            transform: rotate(180deg);
            @include tooltip-arrow;
        }
    }


    @media screen and (max-height: 800px) {

        .create-access {
            padding: 50px 0 20px 0;
            overflow-y: scroll;
        }
    }

    @media screen and (max-height: 750px) {

        .create-access {
            padding: 100px 0 20px 0;
        }
    }

    @media screen and (max-height: 700px) {

        .create-access {
            padding: 150px 0 20px 0;
        }
    }

    @media screen and (max-height: 650px) {

        .create-access {
            padding: 200px 0 20px 0;
        }
    }

    @media screen and (max-height: 600px) {

        .create-access {
            padding: 250px 0 20px 0;
        }
    }

    @media screen and (max-height: 550px) {

        .create-access {
            padding: 300px 0 20px 0;
        }
    }

    @media screen and (max-height: 500px) {

        .create-access {
            padding: 350px 0 20px 0;
        }
    }
</style>
