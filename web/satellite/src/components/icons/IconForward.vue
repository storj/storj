// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M13.7261 4.24989L13.7535 4.27618L18.7238 9.24654C19.0831 9.60579 19.0918 10.1828 18.7501 10.5527L18.7238 10.58L13.7535 15.5504C13.3852 15.9186 12.7882 15.9186 12.42 15.5504C12.0607 15.1911 12.0519 14.6141 12.3937 14.2442L12.42 14.2169L15.7804 10.8562L1.94293 10.8562C1.42217 10.8562 1 10.4341 1 9.91329C1 9.40523 1.40182 8.99101 1.90501 8.97111L1.94293 8.97036L15.7804 8.97029L12.42 5.60969C12.0607 5.25043 12.0519 4.67341 12.3937 4.30353L12.42 4.27618C12.7792 3.91692 13.3562 3.90816 13.7261 4.24989Z" fill="currentColor" />
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
}>(), {
    size: 18,
});
</script>
