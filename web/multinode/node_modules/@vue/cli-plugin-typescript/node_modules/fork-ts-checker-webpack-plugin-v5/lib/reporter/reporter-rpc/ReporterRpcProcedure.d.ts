import { RpcProcedure } from '../../rpc';
import { FilesChange } from '../FilesChange';
import { Issue } from '../../issue';
declare const configure: RpcProcedure<object, void>;
declare const getIssues: RpcProcedure<FilesChange, Issue[]>;
export { configure, getIssues };
