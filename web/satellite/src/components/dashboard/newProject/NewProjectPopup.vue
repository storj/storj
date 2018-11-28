// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="new-project-popup-container">
        <div class="new-project-popup">
            <div class="new-project-popup__info-panel-container">
                <h2 class="new-project-popup__info-panel-container__main-label-text">Create New Project</h2>
                <img src="../../../../static/images/dashboard/CreateNewProject.png" alt="">
            </div>
            <div class="new-project-popup__form-container">
                <HeaderedInput 
                    label="Project Name" 
                    additionalLabel="Up To 20 Characters"
                    placeholder="Enter Project Name" 
                    width="30vw"
                    :error="nameError"
                    @setData="setProjectName">
                </HeaderedInput>
                <HeaderedInput 
                    label="Company Name" 
                    placeholder="Enter Company Name" 
                    width="30vw"
                    @setData="">
                </HeaderedInput>
                <HeaderedInput 
                    label="Description" 
                    placeholder="Enter Project Description" 
                    isMultiline
                    height="10vh"
                    width="30vw"
                    @setData="setProjectDescription">
                </HeaderedInput>
                <div class="new-project-popup__form-container__terms-area">
                    <Checkbox class="new-project-popup__form-container__terms-area__checkbox"
                              @setData="setTermsAccepted"
                              :isCheckboxError="termsAcceptedError"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <div class="new-project-popup__form-container__button-container">
                    <Button label="Cancel" width="14vw" height="48px" :onPress="onCloseClick" isWhite/>
                    <Button label="Create Project" width="14vw" height="48px" :onPress="createProject"/>
                </div>
            </div>
            <div class="new-project-popup__close-cross-container">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderedInput from "@/components/common/HeaderedInput.vue";
import Checkbox from "@/components/common/Checkbox.vue";
import Button from "@/components/common/Button.vue";

@Component(
    { 
        props: {
            onClose: {
                type: Function
            },
            onCreate: {
                type: Function
            }
        },
        data: function() {
            return {
                name: "",
                description: "",
                isTermsAccepted: false,
                termsAcceptedError: false,
                nameError: "",
            }
        },
        methods: {
            setProjectName: function(value: string) : void {
                this.$data.name = value;
                this.$data.nameError = "";
            },
            setProjectDescription: function(value: string) : void {
                this.$data.description = value;
            },
            setTermsAccepted: function(value: boolean) : void {
                this.$data.isTermsAccepted = value;
                this.$data.termsAcceptedError = false;
            },
            onCloseClick: function() : void {
                // TODO: save popup states in store
                this.$emit("onClose");
            },
            createProject: function() : void {
                if (!this.$data.isTermsAccepted) {
                    this.$data.termsAcceptedError = true;
                }

                if (!this.$data.name) {
                    this.$data.nameError = "Name is required!";
                }

                if (this.$data.name.length > 20) {
                    this.$data.nameError = "Name should be less than 21 character!";
                }

                if (this.$data.nameError || this.$data.isTermsAcceptedError) {
                    return;
                }

                this.$store.dispatch("createProject", {
                    name: this.$data.name,
                    description: this.$data.description,
                    isTermsAccepted: this.$data.isTermsAccepted,
                });

                this.$emit("onClose");
            }
        },
        components: {
            HeaderedInput,
            Checkbox,
            Button
        }
    }
)

export default class NewProjectPopup extends Vue {}
</script>

<style scoped lang="scss">
    .new-project-popup-container {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 900;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    .new-project-popup {
        width: 72.3vw;
        height: 76vh;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            height: 55vh;

            &__main-label-text {
                font-family: 'montserrat_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 60px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 32vw;
            margin-left: 5vw;

            &__terms-area {
                display: flex;
                flex-direction: row;
                justify-content: flex-start;
                margin-top: 20px;

                &__checkbox {
                    align-self: center;
                };
                
                h2 {
                    font-family: 'montserrat_regular';
                    font-size: 14px;
                    line-height: 20px;
                    margin-top: 30px;
                    margin-left: 10px;
                };
                a {
                    color: #2683FF;
                    font-family: 'montserrat_bold';
                }
            }

            &__button-container {
                width: 30vw;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
            }
        }

        &__close-cross-container {
            height: 85%;
            width: 1vw;
            display: flex;
            justify-content: center;
            align-items: flex-start;
            margin-left: 3vw;
            svg {
                cursor: pointer;
            }
        }
    }
</style>
