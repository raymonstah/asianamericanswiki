<script>
  import countries from "../lib/flags.json";
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

<style>
  h1 {
    text-align: center;
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
