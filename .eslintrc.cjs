module.exports = {
  root: true,
  extends: ["eslint:recommended", "plugin:svelte/recommended", "prettier"],
  plugins: [],
  // overrides: [{ files: ["*.svelte"], processor: "svelte-eslint-parser" }],
  parserOptions: {
    sourceType: "module",
    ecmaVersion: 2020,
  },
  env: {
    browser: true,
    es2017: true,
    node: true,
  },
};
