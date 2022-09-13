// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="recovery-bar">
        <p v-if="numCodes > 0">
            You only have <b>{{ numCodes }}</b> two-factor authentication recovery code{{ numCodes != 1 ? 's' : '' }} left.
        </p>
        <p v-else>
            You have no more two-factor authentication recovery codes.
        </p>
        <p class="recovery-bar__functional" @click="openGenerateModal">
            Generate new codes.
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class MFARecoveryCodeBar extends Vue {
    @Prop({ default: () => () => {} })
    public readonly openGenerateModal: () => void;

    /**
     * Returns the quantity of MFA recovery codes.
     */
    public get numCodes(): number {
        return this.$store.getters.user.mfaRecoveryCodeCount;
    }
}
</script>

<style scoped lang="scss">
    .recovery-bar {
        width: 100%;
        box-sizing: border-box;
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: #ffc600;
        font-size: 14px;
        line-height: 18px;
        color: #000;
        padding: 5px 30px;

        &__functional {
            font-family: 'font_bold', sans-serif;
            cursor: pointer;
        }
    }
</style>
