import Vue, { PropOptions } from 'vue';
import { Constructor } from 'vue/types/options';
/**
 * decorator of model
 * @param  event event name
 * @param options options
 * @return PropertyDecorator
 */
export declare function Model(event?: string, options?: PropOptions | Constructor[] | Constructor): (target: Vue, key: string) => void;
