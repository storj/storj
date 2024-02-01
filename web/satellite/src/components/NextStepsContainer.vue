// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="!hideContainer">
        <h5 class="text-h5 font-weight-bold">Welcome {{ user.fullName }}!</h5>
        <p class="my-3">Your next steps</p>

        <partner-upgrade-notice-banner v-model="partnerBannerVisible" />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import { User } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';

import PartnerUpgradeNoticeBanner from '@/components/PartnerUpgradeNoticeBanner.vue';

const usersStore = useUsersStore();

const partnerBannerVisible = ref(true);
const hideContainer = ref(false);

const user = computed<User>(() => usersStore.state.user);

// hide container when no content is visible.
watch(partnerBannerVisible, (value) => {
    if (!value) {
        // hide container when no content is visible
        hideContainer.value = true;
    }
});
</script>
