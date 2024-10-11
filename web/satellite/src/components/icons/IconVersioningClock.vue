// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <g v-if="!dotted" clip-path="url(#clip0_5125_630)">
            <path d="M18 0.375H6C2.8934 0.375 0.375 2.8934 0.375 6V18C0.375 21.1066 2.8934 23.625 6 23.625H18C21.1066 23.625 23.625 21.1066 23.625 18V6C23.625 2.8934 21.1066 0.375 18 0.375Z" fill="white" stroke="black" stroke-opacity="0.2" stroke-width="0.75" />
            <path d="M6 12C6 13.1867 6.35189 14.3467 7.01118 15.3334C7.67047 16.3201 8.60754 17.0892 9.7039 17.5433C10.8003 17.9974 12.0067 18.1162 13.1705 17.8847C14.3344 17.6532 15.4035 17.0818 16.2426 16.2426C17.0818 15.4035 17.6532 14.3344 17.8847 13.1705C18.1162 12.0067 17.9974 10.8003 17.5433 9.7039C17.0892 8.60754 16.3201 7.67047 15.3334 7.01118C14.3467 6.35189 13.1867 6 12 6C10.3226 6.00631 8.71265 6.66082 7.50667 7.82667L6 9.33333" stroke="black" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M6 6.85718V9.33336H8.57143" stroke="black" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M12 8.57141V12L15.4286 11.9999" stroke="black" stroke-linecap="round" stroke-linejoin="round" />
        </g>
        <g v-else clip-path="url(#clip0_5112_641)">
            <path d="M18 0.375H6C2.8934 0.375 0.375 2.8934 0.375 6V18C0.375 21.1066 2.8934 23.625 6 23.625H18C21.1066 23.625 23.625 21.1066 23.625 18V6C23.625 2.8934 21.1066 0.375 18 0.375Z" stroke="currentColor" stroke-opacity="0.2" stroke-width="0.75" stroke-dasharray="1.5 1.5" />
            <path d="M6.75 12C6.75 13.0384 7.05791 14.0534 7.63478 14.9167C8.21166 15.7801 9.0316 16.453 9.99091 16.8504C10.9502 17.2477 12.0058 17.3517 13.0242 17.1491C14.0426 16.9466 14.9781 16.4465 15.7123 15.7123C16.4465 14.9781 16.9466 14.0426 17.1491 13.0242C17.3517 12.0058 17.2477 10.9502 16.8504 9.99091C16.453 9.0316 15.7801 8.21166 14.9167 7.63478C14.0534 7.05791 13.0384 6.75 12 6.75C10.5323 6.75552 9.12357 7.32821 8.06833 8.34833L6.75 9.66667" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M6.75 7.5V9.66666H9" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M12 9V12L15 12" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" />
        </g>
        <defs>
            <clipPath id="clip0_5125_630">
                <rect width="24" height="24" fill="white" />
            </clipPath>
        </defs>
    </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
    size?: number | string;
    dotted?: boolean;
}>(), {
    size: 24,
    dotted: false,
});
</script>
