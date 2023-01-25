import preprocess from "svelte-preprocess";
import adapter from "@sveltejs/adapter-static";

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter({
      // default options are shown. On some platforms
      // these options are set automatically — see below
      pages: "build",
      assets: "build",
      fallback: null,
      precompress: false,
      strict: true,
    }),
    prerender: {
      default: true,
    },
    browser: {
      hydrate: true,
      router: true,
    },
    trailingSlash: "always",
  },
  preprocess: [
    preprocess({
      postcss: true,
    }),
  ],
  extensions: [".svelte"],
};

export default config;
