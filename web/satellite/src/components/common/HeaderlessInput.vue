// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-wrap">
        <div class="label-container">
			<img v-if="error" src="../../../static/images/register/ErrorInfo.svg"/>
			<h3 v-if="!error && label" :style="style.labelStyle">{{label}}</h3>
			<h3 class="label-container__error" v-if="error" :style="style.errorStyle">{{error}}</h3>
		</div>
        <input
            v-bind:class="[error ? 'inputError' : null]"
            @input="onInput"
            :placeholder="this.$props.placeholder"
            v-model="value"
            v-bind:type="[isPassword ? passwordType : textType]"
            :style="style.inputStyle"/>
        <!--2 conditions of eye image (crossed or not) -->
            <svg v-if="isPassword && !isPasswordShown" v-on:click="changeVision()" width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M10 4C4.70642 4 1 10 1 10C1 10 3.6999 16 10 16C16.3527 16 19 10 19 10C19 10 15.3472 4 10 4ZM10 13.8176C7.93537 13.8176 6.2946 12.1271 6.2946 10C6.2946 7.87285 7.93537 6.18239 10 6.18239C12.0646 6.18239 13.7054 7.87285 13.7054 10C13.7054 12.1271 12.0646 13.8176 10 13.8176Z" fill="#AFB7C1"/>
                <path d="M11.6116 9.96328C11.6116 10.8473 10.8956 11.5633 10.0116 11.5633C9.12763 11.5633 8.41162 10.8473 8.41162 9.96328C8.41162 9.07929 9.12763 8.36328 10.0116 8.36328C10.8956 8.36328 11.6116 9.07929 11.6116 9.96328Z" fill="#AFB7C1"/>
            </svg>
            <svg v-if="isPassword && isPasswordShown" v-on:click="changeVision()" width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M10 4C4.70642 4 1 10 1 10C1 10 3.6999 16 10 16C16.3527 16 19 10 19 10C19 10 15.3472 4 10 4ZM10 13.8176C7.93537 13.8176 6.2946 12.1271 6.2946 10C6.2946 7.87285 7.93537 6.18239 10 6.18239C12.0646 6.18239 13.7054 7.87285 13.7054 10C13.7054 12.1271 12.0646 13.8176 10 13.8176Z" fill="#AFB7C1"/>
                <path d="M11.6121 9.96328C11.6121 10.8473 10.8961 11.5633 10.0121 11.5633C9.12812 11.5633 8.41211 10.8473 8.41211 9.96328C8.41211 9.07929 9.12812 8.36328 10.0121 8.36328C10.8961 8.36328 11.6121 9.07929 11.6121 9.96328Z" fill="#AFB7C1"/>
                <mask id="path-3-inside-1" fill="white">
                <path fill-rule="evenodd" clip-rule="evenodd" d="M5 16.5L16 1L16.8155 1.57875L5.81551 17.0787L5 16.5Z"/>
                </mask>
                <path fill-rule="evenodd" clip-rule="evenodd" d="M5 16.5L16 1L16.8155 1.57875L5.81551 17.0787L5 16.5Z" fill="white"/>
                <path d="M16 1L16.5787 0.184493L15.7632 -0.394254L15.1845 0.421253L16 1ZM5 16.5L4.18449 15.9213L3.60575 16.7368L4.42125 17.3155L5 16.5ZM16.8155 1.57875L17.631 2.15749L18.2098 1.34199L17.3943 0.76324L16.8155 1.57875ZM5.81551 17.0787L5.23676 17.8943L6.05227 18.473L6.63101 17.6575L5.81551 17.0787ZM15.1845 0.421253L4.18449 15.9213L5.81551 17.0787L16.8155 1.57875L15.1845 0.421253ZM17.3943 0.76324L16.5787 0.184493L15.4213 1.81551L16.2368 2.39425L17.3943 0.76324ZM6.63101 17.6575L17.631 2.15749L16 1L5 16.5L6.63101 17.6575ZM4.42125 17.3155L5.23676 17.8943L6.39425 16.2632L5.57875 15.6845L4.42125 17.3155Z" fill="white" mask="url(#path-3-inside-1)"/>
                <path fill-rule="evenodd" clip-rule="evenodd" d="M5 17.5L16 2L16.8155 2.57875L5.81551 18.0787L5 17.5Z" fill="#AFB7C1"/>
            </svg>
        <!-- end of image-->
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// Custom input component for login page
@Component(
    {
        data: () => {
            return {
                value: '',
                textType: 'text',
                passwordType: 'password',
                isPasswordShown: false
            };
        },
        methods: {
            // Emits data to parent component
            onInput: function(): void {
                this.$emit('setData', this.$data.value);
            },
            // Change condition of password visibility
            changeVision: function(): void {
                this.$data.isPasswordShown = !this.$data.isPasswordShown;
                if (this.$props.isPassword) this.$data.passwordType = this.$data.passwordType == 'password' ? 'text' : 'password';
            },
            setValue(value: string) {
                this.$data.value = value;
            }
        },
        props: {
            placeholder: {
                type: String,
                default: 'default'
            },
            isPassword: {
                type: Boolean,
                default: false
            },
            height: {
                type: String,
                default: '48px'
            },
            width: {
                type: String,
                default: '100%'
            },
            isWhite: {
                type: Boolean,
                default: false
            },
            label: String,
            error: String
        },
        computed: {
            style: function () {
                return {
                    inputStyle: {
                        width: this.$props.width,
                        height: this.$props.height
                    },
                    labelStyle: {
                        color: this.$props.isWhite ? 'white' : '#354049'
                    },
                    errorStyle: {
                        color: this.$props.isWhite ? 'white' : '#FF5560'
                    },
                };
            }
        }
    },
)
export default class HeaderlessInput extends Vue {
}

</script>

<style scoped lang="scss">


input {
	font-family: 'font_regular';
	font-size: 16px;
	line-height: 21px;
	resize: none;
	height: 46px;
	padding: 0;
	width: 100%;
	text-indent: 20px;
	border-color: rgba(56, 75, 101, 0.4);
	border-radius: 6px;
}
input::placeholder {
    color: #384B65;
    opacity: 0.4;
}
.inputError::placeholder {
    color: #EB5757;
    opacity: 0.4;
}
h3 {
	font-family: 'font_regular';
	font-size: 16px;
	line-height: 21px;
	color: #354049;
}
.label-container {
    display: flex;
    justify-content: flex-start;
    flex-direction: row;

    &__add-label {
        margin-left: 5px;
        font-family: 'font_regular';
        font-size: 16px;
        line-height: 21px;
        color: rgba(56, 75, 101, 0.4);
    }

    &__error {
        margin-left: 10px;
    }
}
.error {
	color: #FF5560;
	margin-left: 10px;
}
.input-wrap {
  position: relative;
  width: 100%;

	svg {
		position: absolute;
		right: 15px;
		bottom: 5px;
		transform: translateY(-50%);
		z-index: 20;
		cursor: pointer;

		&:hover path {
			fill: #2683FF !important;
		}
	}
}
</style>
