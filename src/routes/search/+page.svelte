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

<article class="max-w-2xl">
  <div class="relative">
    <div
      class="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none"
    >
      <svg
        aria-hidden="true"
        class="w-5 h-5 text-gray-500 dark:text-gray-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
        ><path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        /></svg
      >
    </div>
    <!-- svelte-ignore a11y-autofocus -->
    <input
      type="search"
      id="searchBar"
      class="block w-full p-4 pl-10 text-2xl text-gray-900 border border-gray-300 rounded bg-gray-50 focus:outline-none focus:ring focus:border-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:placeholder-gray-400 dark:text-white dark:focus:ring-blue-500 dark:focus:border-blue-500"
      placeholder="Search"
      autofocus
      bind:value={query}
      use:debounce={{ query, func: search, duration: 300 }}
      required
    />
  </div>
  <div class="humans">
    {#each hits as hit}
      <div class="human">
        <h1 class="text-xl hover:underline">
          <a class="name" href={"/humans/" + hit.urn_path}>{hit.name}</a>
        </h1>
        <p>{truncate(hit.description, 300)}</p>
      </div>
    {/each}
  </div>
</article>

<style>
  .human {
    padding: 20px 30px 20px 30px;
    margin: 20px;
    background-color: white;
  }
</style>
