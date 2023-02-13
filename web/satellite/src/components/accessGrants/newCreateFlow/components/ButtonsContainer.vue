// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buttons">
        <slot name="leftButton" />
        <slot name="rightButton" />
    </div>
</template>

<style scoped lang="scss">
.buttons {
    display: flex;
    align-items: center;
    column-gap: 16px;
    margin-top: 24px;
}
</style>
