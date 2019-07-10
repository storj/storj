// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="edit-profile-popup-container">
        <div class="edit-profile-popup">
            <div class="edit-profile-popup__form-container">
                <div class="edit-profile-row-container">
                    <div class="edit-profile-popup__form-container__avatar">
                        <h1>{{avatarLetter}}</h1>
                    </div>
                    <h2 class="edit-profile-popup__form-container__main-label-text">Edit profile</h2>
                </div>
                <HeaderedInput
                    class="full-input"
                    label="Full name"
                    placeholder="Enter Full Name"
                    width="100%"
                    ref="fullNameInput"
                    :error="fullNameError"
                    :initValue="originalFullName"
                    @setData="setFullName" />
                <HeaderedInput
                    class="full-input"
                    label="Short Name"
                    placeholder="Enter Short Name"
                    width="100%"
                    ref="shortNameInput"
                    :initValue="originalShortName"
                    @setData="setShortName"/>
                <div class="edit-profile-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite />
                    <Button label="Update" width="205px" height="48px" :onPress="onUpdateClick" />
                </div>
            </div>
            <div class="edit-profile-popup__close-cross-container" @click="onCloseClick">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderedInput from '@/components/common/HeaderedInput.vue';
    import Button from '@/components/common/Button.vue';
    import { USER_ACTIONS, NOTIFICATION_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
		data: function () {
			return {
				originalFullName: this.$store.getters.user.fullName,
				originalShortName: this.$store.getters.user.shortName,

				fullName: this.$store.getters.user.fullName,
				shortName: this.$store.getters.user.shortName,

				fullNameError: '',
			};
		},
		methods: {
			setFullName: function (value: string) {
				this.$data.fullName = value.trim();
				this.$data.fullNameError = '';
			},
			setShortName: function (value: string) {
				this.$data.shortName = value.trim();
			},
			cancel: function () {
				this.$data.fullName = this.$data.originalFullName;
				this.$data.fullNameError = '';
				this.$data.shortName = this.$data.originalShortName;

				let fullNameInput: any = this.$refs['fullNameInput'];
				fullNameInput.setValue(this.$data.originalFullName);

				let shortNameInput: any = this.$refs['shortNameInput'];
				shortNameInput.setValue(this.$data.originalShortName);
			},
			onUpdateClick: async function () {
				if (!this.$data.fullName) {
					this.$data.fullNameError = 'Full name expected';
					return;
				}

				let user = {
					fullName: this.$data.fullName,
					shortName: this.$data.shortName,
				};

				let response = await this.$store.dispatch(USER_ACTIONS.UPDATE, user);
				if (!response.isSuccess) {
					this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
					return;
				}

				this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Account info successfully updated!');

				this.$data.originalFullName = this.$store.getters.user.fullName;
				this.$data.originalShortName = this.$store.getters.user.shortName;
				this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
			},
			onCloseClick: function () {
				(this as any).cancel();
				this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
			}
		},
		computed: {
			avatarLetter: function (): string {
				return this.$store.getters.userName.slice(0, 1).toUpperCase();
			},
		},
		components: {
			HeaderedInput,
			Button,
		}
	})

    export default class EditProfilePopup extends Vue {}
</script>

<style scoped lang="scss">
    p {
        font-family: 'font_medium';
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        display: flex;
    }

    .edit-profile-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: flex-start;
    }
    
    .edit-profile-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1000;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    
    .input-container.full-input {
        width: 100%;
    }

    .edit-profile-popup {
        width: 100%;
        max-width: 440px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px;
        
        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;
            margin-top: 20px;
        }
        
        &__form-container {
            width: 100%;
            max-width: 440px;
            margin-top: 10px;
            
            &__avatar {
                width: 60px;
                height: 60px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #E8EAF2;
                margin-right: 20px;
                
                h1 {
                    font-family: 'font_medium';
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                }
            }
            
            p {
                font-family: 'font_regular';
                font-size: 16px;
                margin-top: 20px;
                
                &:first-child {
                    margin-top: 0;
                }
            }
            
            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 60px;
                color: #384B65;
                margin-top: 0;
            }
            
            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 40px;
            }
        }
        
        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;
            
            &:hover svg path {
                fill: #2683FF;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .edit-profile-popup {
            
            &__info-panel-container {
                display: none;
                
            }
            
            &__form-container {
                
                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>
