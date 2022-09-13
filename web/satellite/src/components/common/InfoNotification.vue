// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isShown" class="notification-wrap">
        <InfoIcon class="notification-wrap__icon" />
        <div class="notification-wrap__text">
            <slot name="text" />
        </div>
        <CloseIcon class="notification-wrap__close" @click="isShown = false" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

// @vue/component
@Component({
    components: {
        CloseIcon,
        InfoIcon,
    },
})
export default class InfoNotification extends Vue {
    public isShown = true;
}
</script>

<style scoped lang="scss">
.notification-wrap {
    position: absolute;
    right: 2.6rem;
    top: 5rem;
    z-index: 9999;
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 1.375rem;
    font-family: 'font_regular', sans-serif;
    background-color: #d7e8ff;
    border: 1px solid #a5beff;
    border-radius: 10px;
    box-shadow: 0 7px 20px rgba(0 0 0 / 15%);
    max-width: 29.6rem;

    &__icon {
        flex-shrink: 0;
        margin-right: 1.375rem;
    }

    &__text {
        max-width: 18.5rem;
        text-align: left;
        word-break: normal;
        font-size: 1rem;
        line-height: 1.625rem;
    }

    &__close {
        margin-left: 2.375rem;
        cursor: pointer;
    }
}

.bold {
    font-family: 'font_bold', sans-serif;
}

.medium {
    font-family: 'font_medium', sans-serif;
}

.link {
    color: black;
    text-decoration: underline !important;
}
</style>
