const { defineConfig } = require('cypress')

module.exports = defineConfig({
  defaultCommandTimeout: 10000,
  e2e: {
    setupNodeEvents(on, config) {
      return require('./cypress/plugins/index.js')(on, config)
    },
  },
})
