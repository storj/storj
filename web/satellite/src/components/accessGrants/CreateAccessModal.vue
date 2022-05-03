// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-access">
        <div class="create-access__modal-container">
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
                    <div>
                        <input type="checkbox" id="acess-grant-check"/>
                        <label for="acess-grant-check">Access Grant</label>
                    </div>
                    <div>
                        <input type="checkbox" id="s3-check"/>
                        <label for="s3-check">S3 Credentials</label>
                    </div>
                    <div>
                        <input type="checkbox" id="cli-check"/>
                        <label for="cli-check">CLI Access</label>
                    </div>
                </div>
                <NameIcon class="create-access__modal-container__body-container__name-icon"/>
                <div class="create-access__modal-container__body-container__name">
                    <p>Name</p>
                    <input type="text" placeholder="Input Access Name" class="create-access__modal-container__body-container__name__input">
                </div>
                <PermissionsIcon class="create-access__modal-container__body-container__permissions-icon"/>
                <div class="create-access__modal-container__body-container__permissions">
                    <p>Permissions</p>
                    <div>
                        <input type="checkbox" id="permissions__all-check"/>
                        <label for="permissions__all-check">All</label>
                        <Chevron @click="togglePermissions" :class="`permissions-chevron-${this.showAllPermissions.position}`"/>
                    </div>
                    <div v-if="this.showAllPermissions.show">
                        <div v-for="(item, key) in permissionsList" v-bind:key="key">
                            <input type="checkbox" :id="`permissions__${item}-check`"/>
                            <label :for="`permissions__${item}-check`">{{item}}</label>
                        </div>
                    </div>
                </div>
                <BucketsIcon class="create-access__modal-container__body-container__buckets-icon"/>
                <div class="create-access__modal-container__body-container__buckets">
                    <p>Buckets</p>
                    <div>
                        <input type="checkbox" id="buckets__all-check"/>
                        <label for="buckets__all-check">All</label>
                        <Chevron 
                        @click="toggleBuckets" 
                        :class="`buckets-chevron-${this.showAllBuckets.position}`"/>
                    </div>
                    <div v-if="this.showAllBuckets.show">
                        <div v-for="(item, key) in bucketsList" v-bind:key="key">
                            <input type="checkbox" :id="`buckets__${item}-check`"/>
                            <label :for="`buckets__${item}-check`">{{item}}</label>
                        </div>
                    </div>
                </div>
                <DateIcon class="create-access__modal-container__body-container__date-icon"/>
                <div class="create-access__modal-container__body-container__end-date">
                    <p>End Date</p>
                    <div>--End Date Picker Here--</div>
                </div>
                <NotesIcon class="create-access__modal-container__body-container__notes-icon"/>
                <div class="create-access__modal-container__body-container__notes">
                    <p>Notes</p>
                    <div>--Notes Section Here--</div>
                </div>
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
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import VButton from '@/components/common/VButton.vue';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import VCheckbox from '@/components/common/VCheckbox.vue';
import TypesIcon from '@/../static/images/accessGrants/create-access_type.svg';
import PermissionsIcon from '@/../static/images/accessGrants/create-access_permissions.svg';
import NameIcon from '@/../static/images/accessGrants/create-access_name.svg';
import BucketsIcon from '@/../static/images/accessGrants/create-access_buckets.svg';
import DateIcon from '@/../static/images/accessGrants/create-access_date.svg';
import NotesIcon from '@/../static/images/accessGrants/create-access_notes.svg';
import Chevron from '@/../static/images/accessGrants/chevron.svg';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';

import { RouteConfig } from '@/router';

// @vue/component
@Component({
    components: {
        CloseCrossIcon,
        VCheckbox,
        VButton,
        TypesIcon,
        PermissionsIcon,
        NameIcon,
        BucketsIcon,
        DateIcon,
        NotesIcon,
        Chevron,
    },
})
export default class CreateAccessModal extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    @Prop({default: () => { return; }})
    private readonly onClose: () => void;

    private showAllPermissions = {show: false, position: "up"};
    private permissionsList = ["Read","Write","List","Delete"];
    private showAllBuckets = {show: false, position: "up"};
    private bucketsList = [];

    /**
     * Closes modal.
     */
    public onCloseClick(): void {
        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR_SELECTION);
        this.$router.push(RouteConfig.AccessGrants.path);
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
     * Toggles buckets list visibility.
     */
    public toggleBuckets(): void {
        if (this.showAllBuckets.show === false) {
            this.showAllBuckets.show = true;
            this.showAllBuckets.position = "down";
        } else {
            this.showAllBuckets.show = false;
            this.showAllBuckets.position = "up";
        }
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
                }
                &__date-icon {
                    grid-column: 1;
                    grid-row: 5;
                } 
                &__end-date {
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

    .buckets-chevron-up {
        @include chevron;
        transform: rotate(-90deg);
    }

    .buckets-chevron-down {
        @include chevron;
    }

    .permissions-chevron-up {
        @include chevron;
        transform: rotate(-90deg);
    }

    .permissions-chevron-down {
        @include chevron;
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
