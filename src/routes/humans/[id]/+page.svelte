<script>
  import SvelteMarkdown from "svelte-markdown";

  export let data;
</script>

<svelte:head>
  <title>{data.human.name} | AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1>{data.human.name}</h1>

  <SvelteMarkdown source={data.human.description} />
</article>
