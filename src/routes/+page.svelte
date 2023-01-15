<script>
  import countries from "../lib/flags.json";
  import algoliasearch from "algoliasearch/lite";
  import { onMount } from "svelte";

  let searchClient;
  let index;

  let query = "";
  let hits = [];

  onMount(() => {
    searchClient = algoliasearch(
      "I3Z39HZCDT",
      "bcefca03d36ddd83a0f2bcb91b8990e7"
    );

    index = searchClient.initIndex("humans");
  });

  // Fires on each keyup in form
  async function search() {
    if (query === "") {
      hits = [];
      return;
    }
    const result = await index.search(query);
    hits = result.hits;
    console.log(hits);
  }
</script>

<svelte:head>
  <title>AsianAmericans.wiki</title>
</svelte:head>

<h1>AsianAmericans.wiki</h1>
<div class="flags">
  {#each Object.entries(countries) as [code, country]}
    <div class="country">
      <span title={country.name} class="emoji">{country.emoji}</span>
    </div>
  {/each}
</div>

<h2>Search</h2>
<div>
  <input type="text" bind:value={query} on:keyup={search} />
</div>
{#each hits as hit}
  <h2><a href={hit.path}>{hit.name}</a></h2>
{/each}

<style>
  h1 {
    text-align: center;
  }

  body {
    font-family: sans-serif;
    padding: 1em;
  }

  .flags {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    grid-template-columns: repeat(5, 1fr);
  }

  .country {
    flex: 0 0 calc(16.66% - 20px);
  }

  .emoji {
    font-size: 100px;
  }
</style>
