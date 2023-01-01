<script>
  import { onMount } from "svelte";

  let humans = [];
  onMount(async () => {
    fetch("https://api-5cwffcuiba-uc.a.run.app/humans/")
      .then((response) => response.json())
      .then((data) => {
        humans = data.data;
      })
      .catch((error) => {
        console.log(error);
      });
  });
</script>

<svelte:head>
  <title>Humans | AsianAmericans.wiki</title>
</svelte:head>

<h1>Asian Americans</h1>
<ul>
  {#each humans as human}
    <li><a href={human.path}>{human.name}</a></li>
  {:else}
    <!-- this block renders when photos.length === 0 -->
    <p>loading...</p>
  {/each}
</ul>
