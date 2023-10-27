// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-medium-emphasis">
        {{ subtitle }}
    </p>
</template>

<script lang="ts">
export default {
    props: {
        subtitle: {
            type: String,
            required: true,
        },
    },
};
</script>