// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-container">
        <div v-if="!isOptional" class="label-container">
            <img v-if="error" src="../../../static/images/register/ErrorInfo.svg"/>
            <h3 v-if="!error">{{label}}</h3>
            <h3  v-if="!error" class="label-container__add-label">{{additionalLabel}}</h3>
            <h3 class="label-container__error" v-if="error">{{error}}</h3>
        </div>
        <div v-if="isOptional" class="optional-label-container">
            <h3>{{label}}</h3>
            <h4>Optional</h4>
        </div>
        <textarea
            v-if="isMultiline"
            :id="this.$props.label"
            :placeholder="this.$props.placeholder"
            :style="style"
            :rows="5"
            :cols="40"
            wrap="hard"
            v-model.lazy="value"
            @change="onInput"
            @input="onInput">
        </textarea>
        <input
            v-if="!isMultiline"
            :id="this.$props.label"
            :placeholder="this.$props.placeholder"
            v-bind:type="[isPassword ? 'password': 'text']"
            v-model.lazy="value"
            @change="onInput"
            @input="onInput"
            :style="style"/>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

	// Custom input component with labeled header
    @Component
    export default class HeaderedInput extends Vue {
        @Prop({default: ''})
        private readonly initValue: string;
        @Prop({default: ''})
        private readonly label: string;
        @Prop({default: ''})
        private readonly additionalLabel: string;
        @Prop({default: 'default'})
        private readonly placeholder: string;
        @Prop({default: ''})
        private readonly error: string;
        @Prop({default: false})
        private readonly isOptional: boolean;
        @Prop({default: false})
        private readonly isMultiline: boolean;
        @Prop({default: false})
        private readonly isPassword: boolean;
        @Prop({default: '48px'})
        private readonly height: string;
        @Prop({default: '100%'})
        private readonly width: string;
        
        private value: string = '';
        
        public constructor() {
            super();
            
            this.value = this.initValue;
        }
        
        public get style() {
            return {
                width: this.width,
                height: this.height,
            };
        }
    
        public onInput() {
            this.$emit('setData', this.$data.value);
        }
        
        public setValue(value: string) {
            this.value = value;
        }
    }
</script>

<style scoped lang="scss">
    .input-container {
    	display: flex;
    	flex-direction: column;
    	align-items: flex-start;
    	margin-top: 10px;
    	width: 48%;
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
            color: #FF5560;
            margin-left: 10px;
    	}
    }
    
    .optional-label-container {
    	display: flex;
    	flex-direction: row;
    	justify-content: space-between;
    	align-items: center;
    	width: 100%;
    
    	h4 {
            font-family: 'font_regular';
            font-size: 16px;
            line-height: 21px;
            color: #AFB7C1;
    	}
    }
    
    input,
    textarea {
    	font-family: 'font_regular';
    	font-size: 16px;
    	line-height: 21px;
    	resize: none;
    	height: 48px;
    	width: 100%;
    	text-indent: 20px;
    	border-color: rgba(56, 75, 101, 0.4);
    	border-radius: 6px;
    	outline: none;
    	box-shadow: none;
    }
    
    textarea {
    	padding-top: 20px;
    }
    
    h3 {
    	font-family: 'font_regular';
    	font-size: 16px;
    	line-height: 21px;
    	color: #354049;
    }
</style>
