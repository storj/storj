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

<script setup lang="ts">
import { computed } from 'vue';

import { useStore } from '@/utils/hooks';

const store = useStore();

const props = withDefaults(defineProps<{
    openGenerateModal: () => void,
}>(), {
    openGenerateModal: () => {},
});

/**
 * Returns the quantity of MFA recovery codes.
 */
const numCodes = computed((): number => {
    return store.getters.user.mfaRecoveryCodeCount;
});
</script>

<style scoped lang="scss">
    .recovery-bar {
        width: 100%;
        box-sizing: border-box;
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: var(--c-yellow-3);
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
