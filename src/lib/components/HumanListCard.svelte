<script>
  import truncateMarkdown from "markdown-truncate";
  import SvelteMarkdown from "svelte-markdown";
  export let path;
  export let name;
  export let description;
</script>

<a
  href={path}
  class="{$$restProps.class || ''}
  block max-w-sm p-6 bg-white border border-gray-200 rounded-lg shadow hover:bg-gray-100 dark:bg-gray-800 dark:border-gray-700 dark:hover:bg-gray-700"
>
  <h2
    class="mb-2 text-2xl font-bold tracking-tight text-gray-900 dark:text-white"
  >
    <a class="name" href={path}>{name}</a>
  </h2>
  <p class="font-normal text-gray-700 dark:text-gray-400 break-words">
    <SvelteMarkdown
      source={truncateMarkdown(description, { limit: 250, ellipsis: true })}
    />
  </p>
</a>
