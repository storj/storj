#!/usr/bin/env node

const commander = require('commander');
const updateNotifier = require('update-notifier');
const pkg = require('../package.json');
const { lint } = require('./lint');

commander.version(pkg.version, '-v, --version');

commander
  .option('-c, --config [config]', 'TSLint configuration file. Default: project root')
  .option('-p, --project [project]', 'Folder that contain your tsconfig.json file. Default: project root')
  .option('--fix', 'Fixes linting errors for select rules (this may overwrite linted files).')
  .option('-e, --exclude <exclude>', 'An array of glob patterns to exclude matches')
  .option('-g, --gitignore', 'Respect ignore patterns in .gitignore files that apply to the globbed files')
  .action((options) => {
    const {
      fix, config, project, exclude, gitignore, log,
    } = options;
    lint({
      fix, config, project, exclude, gitignore, log,
    });
  });

commander.parse(process.argv);

updateNotifier({ pkg }).notify();
