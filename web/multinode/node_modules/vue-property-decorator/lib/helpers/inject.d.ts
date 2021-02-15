import Vue from 'vue';
import { ComponentOptions } from 'vue/types/options';
/** Used for keying reactive provide/inject properties */
export declare const reactiveInjectKey = "__reactiveInject__";
export declare function inheritInjected(componentOptions: ComponentOptions<Vue>): void;
