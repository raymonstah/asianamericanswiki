<script>
  import countries from "../lib/flags.json";

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
</script>

<svelte:head>
  <title>AsianAmericans.wiki</title>
</svelte:head>

<h1 class="text-3xl font-bold">Professions</h1>
<div class="my-4 grid grid-cols-3 gap-3">
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
<h1 class="text-3xl font-bold">Ethnic Backgrounds</h1>
<div class="section">
  {#each Object.entries(countries) as [code, country]}
    <div class="inner-section">
      <span title={country.name} class="emoji"
        ><a class="flagLink" href="/search?query={country.ethnicity}"
          >{country.emoji}</a
        ></span
      >
    </div>
  {/each}
</div>

<style>
  .section {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    grid-template-columns: repeat(5, 1fr);
  }

  .inner-section {
    flex: 0 0 calc(16.66% - 20px);
  }

  .emoji {
    font-size: 100px;
  }

  .flagLink {
    text-decoration: none;
  }
</style>
