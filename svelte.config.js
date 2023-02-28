import preprocess from "svelte-preprocess";
import adapter from "@sveltejs/adapter-static";

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter({
      pages: "build",
      assets: "build",
      fallback: null,
      precompress: false,
      strict: false,
    }),
  },
  preprocess: [
    preprocess({
      postcss: true,
    }),
  ],
  extensions: [".svelte"],
};

export default config;
