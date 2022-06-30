// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="showMessage" class="validation-message__wrapper" :class="{'success-message__wrapper' : isValid, 'error-message__wrapper' : !isValid}">
        <ErrorIcon v-if="!isValid" class="error-message__icon" />
        <p v-if="isValid" class="success-message__text validation-message__text">{{ successMessage }}</p>
        <p v-if="!isValid" class="error-message__text validation-message__text">{{ errorMessage }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ErrorIcon from '@/../static/images/common/errorNotice.svg';

// @vue/component
@Component({
    components: {
        ErrorIcon,
    },
})
export default class ValidationMessage extends Vue {
    @Prop({default: 'Valid!'})
    protected readonly successMessage: string;
    @Prop({default: 'Invalid. Please Try again'})
    protected readonly errorMessage: string;
    @Prop({default: false})
    protected readonly isValid: boolean;
    @Prop({default: false})
    protected readonly showMessage: boolean;
}
</script>

<style scoped lang="scss">
    .validation-message {

        &__wrapper {
            box-sizing: border-box;
            border-radius: 6px;
            width: 100%;
            padding: 12px 25px;
        }

        &__text {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: #1a9666;
            position: relative;
        }
    }

    .success-message {

        &__wrapper {
            background: #eafff7;
            border: 1px solid #1a9666;
        }

        &__text {
            color: #1a9666;
        }
    }

    .error-message {

        &__wrapper {
            background: #fff;
            border: 1px solid #e30011;
            display: flex;
        }

        &__text {
            color: #e30011;
        }

        &__icon {
            position: relative;
            top: 14px;
        }
    }
</style>
