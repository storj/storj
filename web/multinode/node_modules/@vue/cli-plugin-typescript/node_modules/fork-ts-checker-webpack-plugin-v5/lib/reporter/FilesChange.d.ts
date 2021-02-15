interface FilesChange {
    changedFiles?: string[];
    deletedFiles?: string[];
}
/**
 * Computes aggregated files change based on the subsequent files changes.
 *
 * @param changes List of subsequent files changes
 * @returns Files change that represents all subsequent changes as a one event
 */
declare function aggregateFilesChanges(changes: FilesChange[]): FilesChange;
export { FilesChange, aggregateFilesChanges };
