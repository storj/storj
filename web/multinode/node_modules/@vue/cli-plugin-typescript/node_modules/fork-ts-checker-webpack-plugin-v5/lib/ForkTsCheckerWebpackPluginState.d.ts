import { Report } from './reporter';
import { Tap } from 'tapable';
interface ForkTsCheckerWebpackPluginState {
    report: Promise<Report | undefined>;
    removedFiles: string[];
    watching: boolean;
    initialized: boolean;
    webpackDevServerDoneTap: Tap | undefined;
}
declare function createForkTsCheckerWebpackPluginState(): ForkTsCheckerWebpackPluginState;
export { ForkTsCheckerWebpackPluginState, createForkTsCheckerWebpackPluginState };
