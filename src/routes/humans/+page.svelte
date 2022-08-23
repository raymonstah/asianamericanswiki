<script>
  export let data;
</script>

<svelte:head>
  <title>Humans | AsianAmericans.wiki</title>
</svelte:head>

<h1>This is the humans page</h1>
<ul>
  {#each data.humans as human}
    <li>
      <h2>
        <a href={human.path}>
          {human.meta.title}
        </a>
      </h2>
      Published {human.meta.date}
    </li>
  {/each}
</ul>
