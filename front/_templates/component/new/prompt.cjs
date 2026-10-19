const { nameConfig, pathConfig } = require('../../_common/index.cjs');

module.exports = {
  prompt: async ({ prompter }) => {
    const answers = await prompter.prompt([
      nameConfig('component'),
      pathConfig('component'),
    ]);

    return answers;
  },
};
