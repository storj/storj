// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content />
    </VModal>
</template>

<script setup lang="ts">
import { useRouter } from '@/utils/hooks';
import { RouteConfig } from '@/router';

import VModal from '@/components/common/VModal.vue';

const router = useRouter();

/**
 * Closes create access grant flow.
 */
function closeModal(): void {
    router.push(RouteConfig.AccessGrants.path);
}
</script>
