const { createRequire } = require('node:module');

const requireFromHygen = createRequire(require.resolve('hygen'));
const { pascalCase } = requireFromHygen('change-case');

const KEBAB_PATH_REGEX = /^[a-z0-9-](\/?[a-z0-9-])*$/;
const KEBAB_PATH_ERROR = 'The path must only contain lowercase letters, digits, hyphens';

const PAGE_PATH_REGEX = /^[a-z0-9[\]-](\/?[a-z0-9[\]-])*$/;
const PAGE_PATH_ERROR = KEBAB_PATH_ERROR + 'or square brackets';

const hygenIndex = process.argv.findIndex(arg => arg.endsWith('hygen'));
const argv = process.argv.slice(hygenIndex + 1);

const nameConfig = (entity) => {
  return {
    type: 'input',
    name: 'name',
    skip: true,
    default: argv[2],
    validate(value) {
      return pascalCase(value) === value;
    },
  };
};

const pathConfig = (entity, pathRegex = KEBAB_PATH_REGEX, validateMessage = KEBAB_PATH_ERROR) => {
  return {
    type: 'input',
    name: 'path',
    skip: true,
    default: argv[3] || '',
    validate(value) {
      return pascalCase(value) === value;
    },
  };
};

module.exports = {
  PAGE_PATH_REGEX,
  PAGE_PATH_ERROR,
  nameConfig,
  pathConfig,
};
