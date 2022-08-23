<script>
  export let data;
</script>

<svelte:head>
  <title>{data.metadata.title} | AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1>{data.metadata.title}</h1>

  <p>Ethnicity: {data.metadata.ethnicity}</p>
  <p>Published: {data.metadata.date}</p>

  {@html data.content}
</article>
