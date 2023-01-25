<script>
  // used to get the query parameter
  export let data;

  import algoliasearch from "algoliasearch/lite";
  import { onMount } from "svelte";
  import debounce from "../../lib/debounce.js";

  let searchClient;
  let index;

  let query = data.query || "";
  let hits = [];

  onMount(() => {
    searchClient = algoliasearch(
      "I3Z39HZCDT",
      "bcefca03d36ddd83a0f2bcb91b8990e7"
    );

    index = searchClient.initIndex("humans");
    search();
  });

  // Fires on each keyup in form
  async function search() {
    // update the query parameter
    const url = new URL(window.location.toString());
    url.searchParams.set("query", query);
    history.replaceState({}, "", url);
    // perform the query
    if (query === "") {
      hits = [];
      return;
    }
    const result = await index.search(query);
    hits = result.hits;
    console.log(hits);
  }

  function truncate(str, n) {
    return str.length > n ? str.slice(0, n - 1) + "..." : str;
  }
</script>

<svelte:head>
  <title>AsianAmericans.wiki</title>
</svelte:head>

<article>
  <h1>Search</h1>
  <div>
    <!-- svelte-ignore a11y-autofocus -->
    <input
      id="searchBar"
      type="text"
      autofocus
      bind:value={query}
      use:debounce={{ query, func: search, duration: 300 }}
    />
  </div>
  <div class="humans">
    {#each hits as hit}
      <div class="human">
        <h2><a class="name" href={"/humans/" + hit.urn_path}>{hit.name}</a></h2>
        <p>{truncate(hit.description, 300)}</p>
      </div>
    {/each}
  </div>
</article>

<style>
  article {
    max-width: 600px;
  }

  h1 {
    text-align: center;
  }

  .human {
    padding: 20px 30px 20px 30px;
    margin: 20px;
    background-color: white;
  }

  .name {
    color: black;
    text-decoration: none;
  }

  .name:hover {
    text-decoration: underline;
  }

  #searchBar {
    margin: 20px;
    height: 50px;
    font-size: 30px;
  }
</style>
