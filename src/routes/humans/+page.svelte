<script>
  import { onMount } from "svelte";
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import HumanListCard from "../../lib/components/HumanListCard.svelte";
  import InfiniteScroll from "svelte-infinite-scroll";

  let offset = 0;
  let pageSize = 10;
  let humans = [];
  let newBatch = [];
  async function fetchData() {
    await fetch(
      `${PUBLIC_BASE_URL}/humans?offset=${offset}&pageSize=${pageSize}`
    )
      .then((response) => response.json())
      .then((data) => {
        newBatch = data.data;
      })
      .catch((error) => {
        console.log(error);
      });
  }

  onMount(() => {
    // load the first batch
    fetchData();
  });
  $: humans = [...humans, ...newBatch];
</script>

<svelte:head>
  <title>Humans | AsianAmericans.wiki</title>
</svelte:head>

<article class="max-w-2xl">
  <h1 class="text-2xl">Humans</h1>
  <ul>
    {#each humans as human}
      <HumanListCard
        class="my-4"
        path={"/humans/" + human.path}
        description={human.description}
        name={human.name}
      />
    {/each}
  </ul>
  <InfiniteScroll
    hasMore={newBatch.length}
    threshold={pageSize}
    window={true}
    on:loadMore={() => {
      offset += pageSize;
      fetchData();
    }}
  />
</article>
