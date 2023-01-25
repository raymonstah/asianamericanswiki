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

  function truncate(str, n) {
    return str.length > n ? str.slice(0, n - 1) + "..." : str;
  }
</script>

<svelte:head>
  <title>Humans | AsianAmericans.wiki</title>
</svelte:head>

<h1>Humans</h1>
<ul>
  {#each humans as human}
    <div class="human">
      <h2><a class="name" href={human.path}>{human.name}</a></h2>
      <p>{truncate(human.description, 300)}</p>
    </div>
  {:else}
    <!-- this block renders when photos.length === 0 -->
    <p>loading...</p>
  {/each}
</ul>

<style>
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
</style>
