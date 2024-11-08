<template>
    <div class="loader d-flex align-center justify-center">
        <div class="loader__spinner" />
        <img alt="loader" src="@/assets/icon-load.svg" class="loader__icon pb-3">
    </div>
</template>

<style scoped lang="scss">
    @keyframes spin {
        from {
            transform: rotate(0deg);
        }
        to {
            transform: rotate(360deg);
        }
    }

    @keyframes colorChange {
        0% { border-top-color: #0052ff; }
        50% { border-top-color: #091c45 }
        100% { border-top-color: #0052ff; }
    }

    .loader {
        position: absolute;
        inset: 0;

        &__spinner {
            width: 90px;
            height: 90px;
            margin: auto 0;
            border: solid 1px transparent;
            border-radius: 50%;
            animation: 
                spin 0.7s linear infinite,
                colorChange 0.1s linear infinite;
            will-change: transform, border-top-color;
            box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.05);
        }

        &__icon {
            position: absolute;
            inset: 0;
            margin: auto;
        }
    }
</style>