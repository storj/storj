import Vue, { PropOptions } from 'vue';
import { Constructor } from 'vue/types/options';
/**
 * decorator of a synced prop
 * @param propName the name to interface with from outside, must be different from decorated property
 * @param options the options for the synced prop
 * @return PropertyDecorator | void
 */
export declare function PropSync(propName: string, options?: PropOptions | Constructor[] | Constructor): (target: Vue, key: string) => void;
