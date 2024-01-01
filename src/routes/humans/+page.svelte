<script>
  import { onMount } from "svelte";
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import HumanListCard from "../../lib/components/HumanListCard.svelte";
  import InfiniteScroll from "svelte-infinite-scroll";
  import Tags from "svelte-tags-input";
  import ethnicities from "$lib/flags.json";
  import tags from "$lib/tags.json";

  const ethnicityList = Object.values(ethnicities)
    .map((countryData) => countryData.ethnicity)
    .filter((ethnicity) => ethnicity !== undefined);

  let paginated = true;
  let offset = 0;
  let limit = 10;
  let humans = [];
  let newBatch = [];
  let ethnicity = "";
  let minYear = 0;
  let maxYear = 0;
  /**
   * @type {string[]}
   */
  let tagsSelected = [];
  let gender = ""; // one of "male", "female", "nonbinary"

  function convertToYYYYMMDDString(year) {
    const currentDate = new Date();
    const targetDate = new Date(currentDate.getFullYear() - year, 0, 1);

    // Extracting YYYY-MM-DD format
    const yearString = targetDate.getFullYear();
    const monthString = (targetDate.getMonth() + 1).toString().padStart(2, "0");
    const dayString = targetDate.getDate().toString().padStart(2, "0");

    return `${yearString}-${monthString}-${dayString}`;
  }

  // todo: if something changed, reset the offset to 0.
  async function fetchData() {
    // if any of the filters are set, bump the pageSize to 1000.
    if (ethnicity || gender || minYear || maxYear) {
      paginated = false;
      offset = 0;
      limit = 1000;
      humans = [];
    } else {
      paginated = true;
    }
    const queryParams = new URLSearchParams({
      offset: offset,
      limit: limit,
      ethnicity: ethnicity,
      gender: gender,
      olderThan: minYear ? convertToYYYYMMDDString(minYear) : "",
      youngerThan: maxYear ? convertToYYYYMMDDString(maxYear) : "",
    });

    // Add tags only if they are present
    if (tagsSelected.length > 0) {
      tagsSelected.forEach((tag) => {
        queryParams.append("tags", tag.trim());
      });
    }
    const url = `${PUBLIC_BASE_URL}/humans?${queryParams.toString()}`;

    await fetch(url)
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
  $: humans = paginated ? [...humans, ...newBatch] : newBatch;
</script>

<svelte:head>
  <title>Humans | AsianAmericans.wiki</title>
</svelte:head>

<article class="max-w-2xl">
  <h1 class="text-2xl">Humans</h1>

  <div
    class="mt-4 flex flex-wrap gap-y-2 items-center text-gray-700 dark:text-white"
  >
    <div class="w-full">
      <label for="ethnicity" class="block text-sm font-medium">Ethnicity:</label
      >
      <!-- Create a dropdown select input using the ethnicityList -->
      <select
        id="ethnicity"
        class=" dark:text-slate-700 mt-1 p-2 w-full rounded-md shadow-sm border border-gray-300 focus:outline-none focus:ring focus:border-blue-300"
        bind:value={ethnicity}
      >
        <option value="">Any</option>
        {#each ethnicityList as option (option)}
          <option value={option}>{option}</option>
        {/each}
      </select>
    </div>

    <div class="w-1/3">
      <label for="minYear" class="block text-sm font-medium">Min Age:</label>
      <input
        type="number"
        id="minYear"
        class="dark:text-slate-700 mt-1 p-2 w-2/3 rounded-md shadow-sm border border-gray-300 focus:outline-none focus:ring focus:border-blue-300"
        bind:value={minYear}
      />
    </div>

    <div class="w-1/3">
      <label for="maxYear" class="block text-sm font-medium">Max Age:</label>
      <input
        type="number"
        id="maxYear"
        class="dark:text-slate-700 mt-1 p-2 w-2/3 rounded-md shadow-sm border border-gray-300 focus:outline-none focus:ring focus:border-blue-300"
        bind:value={maxYear}
      />
    </div>

    <div class="w-1/3">
      <label for="gender" class="block text-sm font-medium">Gender:</label>
      <select
        id="gender"
        class="dark:text-slate-700 mt-1 p-2 rounded-md shadow-sm border border-gray-300 focus:outline-none focus:ring focus:border-blue-300"
        bind:value={gender}
      >
        <option value="">Any</option>
        <option value="male">Male</option>
        <option value="female">Female</option>
        <option value="nonbinary">Nonbinary</option>
      </select>
    </div>

    <div class="w-full text-slate-700">
      <label for="tags" class="block text-sm font-medium">Tags:</label>
      <Tags
        id="tags"
        name="tags"
        bind:tags={tagsSelected}
        onlyUnique="true"
        maxTags={7}
        autoComplete={tags}
        placeholder={"musician comedian engineer actress"}
      />
    </div>

    <div class="w-full">
      <button
        class="my-4 w-full text-center p-2 rounded-md bg-white dark:text-slate-700 border-gray-500 shadow hover:bg-gray-100"
        on:click={fetchData}>Search</button
      >
    </div>
  </div>

  <div class="my-4 grid grid-cols-1 md:grid-cols-2 gap-3">
    {#each humans as human}
      <HumanListCard
        class="my-4"
        path={"/humans/" + human.path}
        description={human.description}
        name={human.name}
      />
    {/each}
  </div>
  <InfiniteScroll
    hasMore={newBatch.length}
    threshold={limit}
    window={true}
    on:loadMore={() => {
      if (!paginated) {
        return;
      }
      offset += limit;
      fetchData();
    }}
  />
</article>
