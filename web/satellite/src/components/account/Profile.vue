// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="profile-container">
        <div class="profile-container__edit-profile no-margin" >
            <div class="profile-container__edit-profile__row">
                <div class="profile-container__edit-profile__avatar">
                    <h1>{{avatarLetter}}</h1>
                </div>
                <div class="profile-container__edit-profile__text">
                    <h2>Edit Profile</h2>
                    <h3>This information will be visible to all users</h3>
                </div>
            </div>
            <div title="Edit">
                <svg class="edit-svg" width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="toggleEditProfilePopup">
                    <rect width="40" height="40" rx="4" fill="#E2ECF7"/>
                    <path d="M19.0901 21.4605C19.3416 21.7259 19.6695 21.8576 19.9995 21.8576C20.3295 21.8576 20.6574 21.7259 20.9089 21.4605L28.6228 13.3181C29.1257 12.7871 29.1257 11.9291 28.6228 11.3982C28.1198 10.8673 27.3069 10.8673 26.8039 11.3982L19.0901 19.5406C18.5891 20.0715 18.5891 20.9295 19.0901 21.4605ZM27.7134 19.1435C27.0031 19.1435 26.4277 19.7509 26.4277 20.5005V27.2859H13.5713V13.7152H19.9995C20.7097 13.7152 21.2851 13.1078 21.2851 12.3581C21.2851 11.6085 20.7097 11.0011 19.9995 11.0011H13.5713C12.1508 11.0011 11 12.2158 11 13.7152V27.2859C11 28.7852 12.1508 30 13.5713 30H26.4277C27.8482 30 28.999 28.7852 28.999 27.2859V20.5005C28.999 19.7509 28.4236 19.1435 27.7134 19.1435Z" fill="#2683FF"/>
                </svg>
            </div>
        </div>
        <div class="profile-container__secondary-container">
            <div class="profile-container__secondary-container__change-password">
                <div class="profile-container__edit-profile__row">
                    <svg class="profile-container__secondary-container__img" width="60" height="60" viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M30 60C46.5685 60 60 46.5685 60 30C60 13.4315 46.5685 0 30 0C13.4315 0 0 13.4315 0 30C0 46.5685 13.4315 60 30 60Z" fill="#E8EAF2"/>
                        <path d="M29.5001 34.5196C30.1001 34.5196 30.5865 34.0452 30.5865 33.46C30.5865 32.8748 30.1001 32.4004 29.5001 32.4004C28.9 32.4004 28.4136 32.8748 28.4136 33.46C28.4136 34.0452 28.9 34.5196 29.5001 34.5196Z" fill="#354049"/>
                        <path d="M39.9405 40.2152C40.1781 40 40.3139 39.6854 40.3139 39.3709V25.5464C40.3139 24.9007 39.7707 24.3709 39.1086 24.3709H35.7473V21.0927C35.7473 17.7318 32.9462 15 29.5 15C26.0538 15 23.2527 17.7318 23.2527 21.0927V24.3709H19.8914C19.2293 24.3709 18.686 24.9007 18.686 25.5464V39.3709C18.686 39.6854 18.8218 40 19.0595 40.2152L23.7959 44.6689C24.0166 44.8841 24.3222 45 24.6278 45H34.3552C34.6608 45 34.9664 44.8841 35.1871 44.6689L39.9405 40.2152ZM30.7053 36.6391V38.1291C30.7053 38.7748 30.1621 39.3046 29.5 39.3046C28.8379 39.3046 28.2947 38.7748 28.2947 38.1291V36.6391C26.9705 36.1589 26.0198 34.9172 26.0198 33.4437C26.0198 31.5728 27.5817 30.0497 29.5 30.0497C31.4183 30.0497 32.9801 31.5728 32.9801 33.4437C32.9801 34.9172 32.0295 36.1589 30.7053 36.6391ZM33.3367 24.3709H25.6464V21.0927C25.6464 19.0232 27.3779 17.351 29.483 17.351C31.5881 17.351 33.3197 19.0397 33.3197 21.0927V24.3709H33.3367Z" fill="#354049"/>
                    </svg>
                    <div class="profile-container__secondary-container__change-password__text-container">
                        <h2>Change Password</h2>
                        <h3>6 or more characters</h3>
                    </div>
                </div>
                <div title="Edit">
                    <svg class="edit-svg" width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="toggleChangePasswordPopup">
                        <rect width="40" height="40" rx="4" fill="#E2ECF7"/>
                        <path d="M19.0901 21.4605C19.3416 21.7259 19.6695 21.8576 19.9995 21.8576C20.3295 21.8576 20.6574 21.7259 20.9089 21.4605L28.6228 13.3181C29.1257 12.7871 29.1257 11.9291 28.6228 11.3982C28.1198 10.8673 27.3069 10.8673 26.8039 11.3982L19.0901 19.5406C18.5891 20.0715 18.5891 20.9295 19.0901 21.4605ZM27.7134 19.1435C27.0031 19.1435 26.4277 19.7509 26.4277 20.5005V27.2859H13.5713V13.7152H19.9995C20.7097 13.7152 21.2851 13.1078 21.2851 12.3581C21.2851 11.6085 20.7097 11.0011 19.9995 11.0011H13.5713C12.1508 11.0011 11 12.2158 11 13.7152V27.2859C11 28.7852 12.1508 30 13.5713 30H26.4277C27.8482 30 28.999 28.7852 28.999 27.2859V20.5005C28.999 19.7509 28.4236 19.1435 27.7134 19.1435Z" fill="#2683FF"/>
                    </svg>
                </div>
            </div>
            <div class="profile-container__secondary-container__email-container">
                <div class="profile-container__edit-profile__row">
                    <svg class="profile-container__secondary-container__img" width="60" height="60" viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M30 60C46.5685 60 60 46.5685 60 30C60 13.4315 46.5685 0 30 0C13.4315 0 0 13.4315 0 30C0 46.5685 13.4315 60 30 60Z" fill="#E8EAF2"/>
                        <path d="M29.9998 16.7598C25.7965 16.7641 21.8459 18.7646 19.3534 22.1498C16.8607 25.535 16.1236 29.9013 17.3671 33.9162C18.6096 37.9314 21.6853 41.1186 25.6546 42.5032C29.6238 43.8868 34.014 43.305 37.4846 40.9348L35.9861 38.7538C33.3633 40.552 30.0711 41.0781 27.0174 40.1856C23.9649 39.2931 21.4732 37.077 20.2319 34.1482C18.9905 31.2203 19.1294 27.8888 20.6104 25.0734C22.0924 22.2591 24.7588 20.2577 27.8761 19.6232C30.9922 18.9878 34.2296 19.7862 36.6938 21.7976C39.1581 23.809 40.5898 26.8202 40.592 30.0007V31.0365C40.592 32.8762 39.8001 33.9732 38.4723 33.9732V33.995L38.4734 33.9961C38.3892 33.9819 38.305 33.9743 38.2208 33.9743H37.9408C37.1959 33.9743 36.0868 32.8903 36.0868 32.1597V30.0007C36.0989 28.1019 35.2293 26.3038 33.7331 25.1345C32.2358 23.9653 30.2824 23.5563 28.4417 24.0277C26.602 24.498 25.085 25.7963 24.3335 27.5408C23.5832 29.2854 23.6838 31.2794 24.6069 32.9395C25.529 34.5987 27.1696 35.7373 29.0475 36.0206C30.9254 36.3039 32.8286 35.6991 34.2 34.3855C35.0597 35.6542 36.521 36.6255 37.9386 36.6255V36.6025C38.0228 36.6178 38.1092 36.6255 38.1956 36.6255H38.4756C41.326 36.6255 43.2356 34.3855 43.2356 31.042V30.0007C43.2312 26.4909 41.8356 23.1265 39.3551 20.6448C36.8734 18.1631 33.5101 16.7654 30.0003 16.7598L29.9998 16.7598ZM29.9998 33.4416C28.6075 33.4416 27.353 32.6038 26.8204 31.3176C26.2878 30.0313 26.582 28.5515 27.5664 27.567C28.5508 26.5824 30.0306 26.2884 31.317 26.821C32.6032 27.3537 33.441 28.6082 33.441 30.0004C33.441 31.9014 31.901 33.4413 30.0001 33.4413L29.9998 33.4416Z" fill="#354049"/>
                    </svg>
                    <div class="profile-container__secondary-container__email-container__text-container">
                        <h2>{{user.email}}</h2>
                    </div>
                </div>
            </div>
        </div>
        <div class="profile-container__button-area">
            <Button
                class="profile-container__button-area__delete-button"
                label="Delete account"
                width="210px"
                height="56px"
                :onPress="toggleDeleteAccountPopup"
                isDeletion="true"/>
        </div>
        <ChangePasswordPopup v-if="isChangePasswordPopupShown"/>
        <EditProfilePopup v-if="isEditProfilePopupShown"/>
        <DeleteAccountPopup v-if="isDeleteAccountPopupShown" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { APP_STATE_ACTIONS, USER_ACTIONS } from '@/utils/constants/actionNames';
    import DeleteAccountPopup from '@/components/account/DeleteAccountPopup.vue';
    import ChangePasswordPopup from '@/components/account/ChangePasswordPopup.vue';
    import EditProfilePopup from '@/components/account/EditProfilePopup.vue';
    import { User } from '../../types/users';

    @Component({
        components: {
            Button,
            DeleteAccountPopup,
            ChangePasswordPopup,
            EditProfilePopup,
        },
    })
    export default class Profile extends Vue {
        public mounted(): void {
            this.$store.dispatch(USER_ACTIONS.GET);
        }

        public toggleDeleteAccountPopup(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
        }
        public toggleChangePasswordPopup(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
        }
        public toggleEditProfilePopup(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
        }

        public get user(): User {
            return this.$store.getters.user;
        }
        public get isEditProfilePopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isEditProfilePopupShown;
        }
        public get isChangePasswordPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isChangePasswordPopupShown;
        }
        public get isDeleteAccountPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isDeleteAccountPopupShown;
        }
        public get avatarLetter(): string {
            return this.$store.getters.userName.slice(0, 1).toUpperCase();
        }
    }
</script>

<style scoped lang="scss">
    .profile-container {
        position: relative;
        margin-top: 83px;
        
        &__navigation {
            position: absolute;
            right: 55px;
            top: 44px;
            z-index: 99;
        }
        
        h1 {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
        
        h2 {
            @extend h1;
            font-size: 18px;
            line-height: 27px;
            word-break: break-all;
        }
        
        h3 {
            @extend h2;
            font-family: 'font_regular';
            color: #AFB7C1;
            font-size: 16px;
            line-height: 21px;
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
                background: #E8EAF2;
                margin-right: 32px;
            
                h1 {
                    font-family: 'font_medium';
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
                width: 60px;
                height: 60px;
            }
        }
        
        &__button-area {
            margin-top: 40px;
            width: 100%;
            align-self: flex-start;
            align-items: center;
            display: flex;
            flex-direction: row;
            justify-content: flex-start;
        }
    }
    
    .no-margin {
        margin-top: 0;
    }
    
    .edit-svg {
        cursor: pointer;
    
        &:hover {
        
            rect {
                fill: #2683FF !important;
            }
            
            path {
                fill: white !important;
            }
        }
    }
    
    .input-container.full-input,
    .input-wrap.full-input {
        width: 100%;
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
            
            margin-top: 70px;
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
