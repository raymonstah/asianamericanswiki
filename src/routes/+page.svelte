<script>
  import countries from "../lib/flags.json";
  import HumanListCard from "../lib/components/HumanListCard.svelte";

  let professions = [
    "Actor",
    "Athlete",
    "Author",
    "Comedian",
    "Entrepreneur",
    "Model",
    "Musician",
    "Producer",
    "YouTuber",
  ];

  export let data;
</script>

<svelte:head>
  <title>AsianAmericans.wiki</title>
</svelte:head>

{#if data.mostViewedHumans && data.mostViewedHumans.length > 0}
  <h1 class="text-3xl font-bold">Most Viewed</h1>
  <div class="my-4 grid grid-cols-1 md:grid-cols-2 gap-3">
    {#each data.mostViewedHumans as human}
      <HumanListCard
        class="my-4"
        path={"/humans/" + human.path}
        description={human.description}
        name={human.name}
      />
    {/each}
  </div>
{/if}

<h1 class="text-3xl font-bold">Ethnic Backgrounds</h1>
<div class="my-4 grid grid-cols-3 md:grid-cols-6 gap-3">
  {#each Object.entries(countries) as [code, country]}
    <div>
      <span title={country.name} class="text-9xl"
        ><a href="/search?query={country.ethnicity}">{country.emoji}</a></span
      >
    </div>
  {/each}
</div>

<h1 class="text-3xl font-bold">Professions</h1>
<div class="my-4 grid grid-cols-2 md:grid-cols-3 gap-3">
  {#each professions as profession}
    <div class="max-w-sm rounded overflow-hidden shadow-lg">
      <div class="px-6 py-4">
        <div class="font-bold text-xl mb-2">
          <a href="/search?query={profession}">{profession}</a>
        </div>
      </div>
    </div>
  {/each}
</div>
