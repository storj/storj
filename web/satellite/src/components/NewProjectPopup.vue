<template>
    <div class="npPopupContainer">
        <div class="npPopup">
            <div class="npInfoPanelContainer">
                <h2 class="npMainLabelText">Create New Project</h2>
                <img src="../../static/images/dashboard/CreateNewProject.png" alt="">
            </div>
            <div class="npFormContainer">
                <HeaderedInput 
                    label="Project Name" 
                    additionalLabel="Up To 20 Characters"
                    placeholder="Enter Project Name" 
                    width="30vw"
                    @setData="setProjectName">
                </HeaderedInput>
                <HeaderedInput 
                    label="Company Name" 
                    placeholder="Enter Company Name" 
                    width="30vw"
                    @setData="setCompanyName">
                </HeaderedInput>
                <HeaderedInput 
                    label="Description" 
                    placeholder="Enter Project Description" 
                    isMultiline
                    height="10vh"
                    width="30vw"
                    @setData="setDescription">
                </HeaderedInput>
                <div class="npTermsArea">
                    <Checkbox class="npCheckBox" @setData="setTermsAccepted"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <div class="npButtonContainer">
                    <Button label="Cancel" width="14vw" height="48px" :onPress="onCloseClick" isWhite/>
                    <Button label="Create Project" width="14vw" height="48px" :onPress="onCreate"/>
                </div>
            </div>
            <div class="closeCrossContainer">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderedInput from "./HeaderedInput.vue";
import Checkbox from "./Checkbox.vue";
import Button from "./Button.vue";

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
        methods: {
            onCloseClick: function () : void {
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
    .npPopupContainer {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 100;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    .npPopup {
        width: 72.3vw;
        height: 76vh;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
    }
    .npInfoPanelContainer {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
        align-items: center;
        height: 55vh;
    }
    .npMainLabelText {
        font-family: 'montserrat_bold';
        font-size: 32px;
        line-height: 39px;
        color: #384B65;
        margin-bottom: 60px;
        margin-top: 0;
    }
    .npFormContainer {
        width: 32vw;
        margin-left: 5vw;
    }
    .npTermsArea {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		margin-top: 20px;
		.npCheckBox {
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
    .npButtonContainer {
        width: 30vw;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        margin-top: 30px;
    }
    .closeCrossContainer {
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
</style>
