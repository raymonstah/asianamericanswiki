import { sveltekit } from "@sveltejs/kit/vite";
import { plugin as markdown } from "vite-plugin-markdown";
/** @type {import('vite').UserConfig} */
const config = {
  plugins: [markdown({ mode: ["html", "toc"] }), sveltekit()],
};

export default config;
