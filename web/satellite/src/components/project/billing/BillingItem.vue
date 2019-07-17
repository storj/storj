// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr class="container">
        <td class="container__item bold">test</td>
        <td class="container__item">test</td>
        <td class="container__item">
            <div class="row justify-start">
                <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M20 7.26316V3.87134C20 3.40351 19.7938 2.93567 19.4845 2.5848C19.1753 2.23392 18.7629 2 18.3505 2H1.64948C1.23711 2 0.824742 2.23392 0.515464 2.5848C0.206186 2.93567 0 3.40351 0 3.87134V7.26316H20Z" fill="#2683FF"/>
                    <path d="M0 9.36816V16.1852C0 16.5862 0.206186 16.9872 0.515464 17.288C0.824742 17.5887 1.23711 17.7892 1.64948 17.7892H18.3505C18.7629 17.7892 19.1753 17.5887 19.4845 17.288C19.7938 16.9872 20 16.5862 20 16.1852V9.36816H0ZM5.36083 15.1827H2.68041V13.8794H5.36083V15.1827ZM10.7217 15.1827H6.70103V13.8794H10.7217V15.1827Z" fill="#2683FF"/>
                </svg>
                <p>test</p>
            </div>
        </td>
        <td class="container__item">
            <div class="row justify-start">
                <svg width="16" height="12" viewBox="0 0 16 12" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M13.3333 0H2.66667C1.19418 0 0 1.15147 0 2.57131V9.42869C0 10.8485 1.19418 12 2.66667 12H13.3333C14.8058 12 16 10.8485 16 9.42869V2.57131C16 1.15147 14.8058 0 13.3333 0ZM11.1383 8.1165C11.2642 8.23703 11.335 8.40095 11.335 8.57209C11.335 8.74244 11.265 8.90636 11.14 9.02769C11.0142 9.14822 10.8442 9.21571 10.6675 9.21571C10.49 9.21571 10.32 9.14742 10.195 9.02608L8.00004 6.90876L5.80506 9.02608C5.54422 9.27517 5.12339 9.27437 4.86338 9.02448C4.60423 8.77376 4.60337 8.36799 4.86172 8.11647L7.05756 5.99997L4.86172 3.88348C4.60339 3.63196 4.60422 3.22619 4.86338 2.97547C5.12339 2.72558 5.54422 2.72476 5.80506 2.97387L8.00004 5.09119L10.195 2.97469V2.97389C10.4559 2.7248 10.8767 2.7256 11.1367 2.97549C11.3959 3.22621 11.3967 3.63198 11.1384 3.8835L8.94253 5.99999L11.1383 8.1165Z" fill="#EB001B"/>
                </svg>
                <p>test</p>
            </div>
        </td>
        <td class="container__item bold">
            <div class="row">
                <p>test</p>
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect class="background" width="40" height="40" rx="4" fill="#E2ECF7"/>
                    <path class="blue" d="M25.6491 19.2809L21.2192 23.5281C20.891 23.8427 20.3988 24 20.0707 24C19.7425 24 19.2503 23.8427 19.0862 23.5281L14.4922 19.2809C13.8359 18.6517 13.8359 17.8652 14.4922 17.236C14.8204 16.9213 15.1485 16.9213 15.6407 16.9213C15.9689 16.9213 16.4611 17.0787 16.6252 17.3933L18.594 19.1236L18.594 11.4157C18.594 10.6292 19.2503 10 20.0707 10C20.891 10 21.5473 10.6292 21.5473 11.4157L21.5473 19.1236L23.5162 17.236C23.6803 16.9213 24.1725 16.9213 24.5006 16.9213C24.8288 16.9213 25.321 17.0787 25.4851 17.3933C26.1414 17.8652 26.1414 18.809 25.6491 19.2809Z" fill="#2683FF"/>
                    <rect class="blue" x="11" y="28" width="18" height="2" rx="1" fill="#2683FF"/>
                </svg>
            </div>
        </td>
    </tr>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component({})

    export default class BillingItem extends Vue {}
</script>

<style scoped lang="scss">
    .container {
        transition: box-shadow .2s ease-out;
        padding: 35px 0px 35px 70px;
        transition: all .2s ease;
        margin-bottom: 10px;
        border-radius: 6px;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        outline: none;
        
        &:hover {
            background: #fff;
            box-shadow: 0px 4px 4px rgba(231, 232, 238, 0.6);
        }
        
        &__item {
            width: 20%;
            height: 80px;
            padding-left: 30px;
            font-family: 'font_medium';
            font-size: 16px;
            margin: 0;
            
            svg {
                cursor: pointer;
            }
            
            svg:hover {
            
                .background {
                    fill: #2683FF !important;
                }
	             
                .blue {
                    fill: white !important;
                }
            }
        }
        
        &:first-of-type {
            font-weight: bold;
        }
    }
    
    .row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding-right: 36px;
    }
    
    .justify-start {
        justify-content: flex-start;
        
        svg {
            cursor: default;
        }
        
        p {
            margin-left: 7px;
        }
    }
    
    .bold {
        font-family: 'font_bold';
    }
    
    @media screen and (max-width: 1600px) {
        .container {
            grid-template-columns: 2% 20% 30% 20% 15% 13%;
            padding: 20px 0px 20px 70px;
        }
    }
</style>
