<script>
  import { onMount } from "svelte";

  export let humans;

  onMount(() => {
    console.log("In Gallery.svelte", humans);
  });
</script>

<div class="columns-2 md:columns-3 gap-0">
  {#each humans as human}
    <a href="/humans/{human.path}">
      <img
        class="h-auto max-w-full"
        src={human.featuredImage}
        title={human.name}
        alt={human.name}
      />
    </a>
  {:else}
    <p>{humans.length}</p>
    <p>Loading or no data available</p>
  {/each}
</div>
