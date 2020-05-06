// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="profile-container">
        <h1 class="profile-container__title">Account Settings</h1>
        <div class="profile-container__edit-profile no-margin" >
            <div class="profile-container__edit-profile__row">
                <div class="profile-container__edit-profile__avatar">
                    <h1 class="profile-container__edit-profile__avatar__letter">{{avatarLetter}}</h1>
                </div>
                <div class="profile-container__edit-profile__text">
                    <h2 class="profile-bold-text">Edit Profile</h2>
                    <h3 class="profile-regular-text">This information will be visible to all users</h3>
                </div>
            </div>
            <EditIcon
                class="edit-svg"
                @click="toggleEditProfilePopup"
            />
        </div>
        <div class="profile-container__secondary-container">
            <div class="profile-container__secondary-container__change-password">
                <div class="profile-container__edit-profile__row">
                    <ChangePasswordIcon class="profile-container__secondary-container__img"/>
                    <div class="profile-container__secondary-container__change-password__text-container">
                        <h2 class="profile-bold-text">Change Password</h2>
                        <h3 class="profile-regular-text">6 or more characters</h3>
                    </div>
                </div>
                <EditIcon
                    class="edit-svg"
                    @click="toggleChangePasswordPopup"
                />
            </div>
            <div class="profile-container__secondary-container__email-container">
                <div class="profile-container__edit-profile__row">
                    <EmailIcon class="profile-container__secondary-container__img"/>
                    <div class="profile-container__secondary-container__email-container__text-container">
                        <h2 class="profile-bold-text email">{{user.email}}</h2>
                    </div>
                </div>
            </div>
        </div>
        <ChangePasswordPopup v-if="isChangePasswordPopupShown"/>
        <EditProfilePopup v-if="isEditProfilePopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ChangePasswordPopup from '@/components/account/ChangePasswordPopup.vue';
import DeleteAccountPopup from '@/components/account/DeleteAccountPopup.vue';
import EditProfilePopup from '@/components/account/EditProfilePopup.vue';
import VButton from '@/components/common/VButton.vue';

import ChangePasswordIcon from '@/../static/images/account/profile/changePassword.svg';
import EmailIcon from '@/../static/images/account/profile/email.svg';
import EditIcon from '@/../static/images/common/edit.svg';

import { USER_ACTIONS } from '@/store/modules/users';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        EditIcon,
        ChangePasswordIcon,
        EmailIcon,
        VButton,
        DeleteAccountPopup,
        ChangePasswordPopup,
        EditProfilePopup,
    },
})
export default class SettingsArea extends Vue {
    /**
     * Lifecycle hook after initial render where user info is fetching.
     */
    public mounted(): void {
        this.$store.dispatch(USER_ACTIONS.GET);
    }

    /**
     * Opens delete account popup.
     */
    public toggleDeleteAccountPopup(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
    }

    /**
     * Opens change password popup.
     */
    public toggleChangePasswordPopup(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
    }

    /**
     * Opens edit account info popup.
     */
    public toggleEditProfilePopup(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
    }

    /**
     * Returns user info from store.
     */
    public get user(): User {
        return this.$store.getters.user;
    }

    /**
     * Indicates if edit user info popup is shown.
     */
    public get isEditProfilePopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isEditProfilePopupShown;
    }

    /**
     * Indicates if change password popup is shown.
     */
    public get isChangePasswordPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isChangePasswordPopupShown;
    }

    /**
     * Indicates if delete account popup is shown.
     */
    public get isDeleteAccountPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isDeleteAccountPopupShown;
    }

    /**
     * Returns first letter of user name.
     */
    public get avatarLetter(): string {
        return this.$store.getters.userName.slice(0, 1).toUpperCase();
    }
}
</script>

<style scoped lang="scss">
    .profile-container {
        position: relative;
        font-family: 'font_regular', sans-serif;
        padding-bottom: 100px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin: 40px 0 25px 0;
        }

        &__edit-profile {
            height: 66px;
            width: calc(100% - 80px);
            border-radius: 6px;
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            padding: 37px 40px;
            margin-top: 40px;
            background-color: #fff;

            &__row {
                display: flex;
                flex-direction: row;
                justify-content: flex-start;
                align-items: center;
            }

            &__avatar {
                width: 60px;
                height: 60px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #e8eaf2;
                margin-right: 32px;

                &__letter {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                }
            }
        }

        &__secondary-container {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            margin-top: 40px;

            &__change-password {
                height: 66px;
                border-radius: 6px;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                padding: 37px 40px;
                background-color: #fff;
                width: calc(48% - 80px);

                &__text-container {
                    margin-left: 32px;
                }
            }

            &__email-container {
                height: 66px;
                border-radius: 6px;
                display: flex;
                flex-direction: row;
                justify-content: flex-start;
                align-items: center;
                padding: 37px 40px;
                background-color: #fff;
                width: calc(48% - 80px);

                &__text-container {
                    margin-left: 32px;
                }
            }

            &__img {
                min-width: 60px;
                min-height: 60px;
            }
        }
    }

    .no-margin {
        margin-top: 0;
    }

    .edit-svg {
        cursor: pointer;

        &:hover {

            .edit-svg__rect {
                fill: #2683ff;
            }

            .edit-svg__path {
                fill: white;
            }
        }
    }

    .input-container.full-input,
    .input-wrap.full-input {
        width: 100%;
    }

    .profile-bold-text {
        font-family: 'font_bold', sans-serif;
        color: #354049;
        margin-block-start: 0.5em;
        margin-block-end: 0.5em;
        font-size: 18px;
        line-height: 27px;
        word-break: break-all;
        max-height: 80px;
    }

    .profile-regular-text {
        margin-block-start: 0.5em;
        margin-block-end: 0.5em;
        color: #afb7c1;
        font-size: 16px;
        line-height: 21px;
    }

    .email {
        user-select: text;
    }

    @media screen and (max-width: 1300px) {

        .profile-container {

            &__secondary-container {
                flex-direction: column;
                justify-content: center;

                &__change-password {
                    width: calc(100% - 80px);
                }

                &__email-container {
                    margin-top: 40px;
                    width: calc(100% - 80px);
                }
            }
        }
    }

    @media screen and (max-height: 825px) {

        .profile-container {

            &__secondary-container {
                margin-top: 20px;

                &__email-container {
                    margin-top: 20px;
                }
            }

            &__button-area {
                margin-top: 20px;
            }
        }
    }

    @media screen and (max-height: 790px) {

        .profile-container {
            height: 535px;
            overflow-y: scroll;

            &::-webkit-scrollbar,
            &::-webkit-scrollbar-track,
            &::-webkit-scrollbar-thumb {
                visibility: hidden;
            }
        }
    }

    @media screen and (max-height: 770px) {

        .profile-container {
            height: 515px;
        }
    }

    @media screen and (max-height: 750px) {

        .profile-container {
            height: 495px;
        }
    }

    @media screen and (max-height: 730px) {

        .profile-container {
            height: 475px;
        }
    }

    @media screen and (max-height: 710px) {

        .profile-container {
            height: 455px;
        }
    }
</style>
