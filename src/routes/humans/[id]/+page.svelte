<script>
  import { PUBLIC_BASE_URL } from "$env/static/public";
  import SvelteMarkdown from "svelte-markdown";
  import Chip from "../../../lib/components/Chip.svelte";
  import Affiliate from "../../../lib/components/Affiliate.svelte";
  import dayjs from "dayjs";
  import relativeTime from "dayjs/plugin/relativeTime";
  import { getAuth, onAuthStateChanged } from "firebase/auth";
  dayjs.extend(relativeTime);

  export let data;

  const humanFields = [
    { key: "aliases", label: "Aliases", isArray: true },
    { key: "dob", label: "Born", isDate: true },
    { key: "dod", label: "Died", isDate: true },
    { key: "ethnicity", label: "Ethnicity", isArray: true },
    { key: "birthLocation", label: "Birth Location" },
    { key: "location", label: "Location", isArray: true },
    { key: "createdAt", label: "Created", isRelativeDate: true },
    { key: "updatedAt", label: "Last Updated", isRelativeDate: true },
  ];

  function formatDateString(dateString) {
    const parts = dateString.split("-");
    const year = parts[0];
    const month = parts[1];
    const day = parts[2];

    if (year && month && day) {
      return dayjs(dateString).format("MMMM D, YYYY");
    } else if (year && month) {
      return `${dayjs(dateString).format("MMMM")}, ${year}`;
    } else if (year) {
      return year;
    } else {
      return "";
    }
  }

  let isHumanSaved = false;
  function saveHuman() {
    isHumanSaved = !isHumanSaved;
    if (isHumanSaved) {
      const auth = getAuth();
      onAuthStateChanged(auth, function (user) {
        if (user) {
          user.getIdToken().then(function (token) {
            const headers = new Headers({
              Authorization: `Bearer ${token}`,
            });
            fetch(`${PUBLIC_BASE_URL}/humans/${data.human.id}/save`, {
              method: "POST",
              headers: headers,
            }).catch((error) => {
              console.error("Error:", error);
            });
          });
        } else {
          alert("You must be logged in to save a human.");
        }
      });
    }
  }
</script>

<article class="max-w-2xl">
  <!-- Header -->
  <h1 class="text-2xl">{data.human.name}</h1>
  <button class="cursor-pointer" on:click={saveHuman}>
    <div class="my-2">
      <svg
        fill="currentColor"
        class={isHumanSaved ? "text-amber-300 " : "text-gray-500 "}
        xmlns="http://www.w3.org/2000/svg"
        height="2em"
        viewBox="0 0 512 512"
        ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
          d="M47.6 300.4L228.3 469.1c7.5 7 17.4 10.9 27.7 10.9s20.2-3.9 27.7-10.9L464.4 300.4c30.4-28.3 47.6-68 47.6-109.5v-5.8c0-69.9-50.5-129.5-119.4-141C347 36.5 300.6 51.4 268 84L256 96 244 84c-32.6-32.6-79-47.5-124.6-39.9C50.5 55.6 0 115.2 0 185.1v5.8c0 41.5 17.2 81.2 47.6 109.5z"
        /></svg
      >
    </div>
  </button>
  <!-- Table -->
  <table
    class="table-fixed w-full text-sm text-left text-gray-500 dark:text-white bg-gray-200 dark:bg-gray-800"
  >
    {#each humanFields as field}
      {#if data.human[field.key]}
        <tr
          class="border-b dark:bg-gray-800 border-gray-300 dark:border-gray-700"
        >
          <th
            class="dark:text-white px-4 py-4 w-1/3 font-medium text-gray-900 whitespace-nowrap"
            >{field.label}</th
          >
          <td
            class="dark:text-white px-4 py-4 w-2/3 font-medium text-gray-900 whitespace-nowrap"
          >
            {#if field.isArray}
              <ul class="flex flex-row flex-wrap gap-y-3">
                {#each data.human[field.key] as item}
                  <li class="space-y-4">
                    <Chip><a href="/search?query={item}">{item}</a></Chip>
                  </li>
                {/each}
              </ul>
            {:else if field.isRelativeDate}
              {dayjs(data.human[field.key]).fromNow()}
            {:else if field.isDate}
              {#if data.human[field.key]}
                {formatDateString(data.human[field.key])}
                <!-- parse the "dob" to determine how old they are assuming "dod" is not present. -->
                {#if data.human.dob && !data.human.dod}
                  (age {dayjs().diff(data.human.dob, "year")} years)
                {/if}
              {/if}
            {:else}
              {data.human[field.key]}
            {/if}
          </td>
        </tr>
      {/if}
    {/each}

    <!-- Socials Row -->
    <tr class="dark:bg-gray-800">
      <th
        class="dark:text-white px-4 py-4 w-1/3 font-medium text-gray-900 whitespace-nowrap"
        >Socials</th
      >
      <td
        class="dark:text-white px-4 py-4 w-2/3 font-medium text-gray-900 whitespace-nowrap"
      >
        <ul class="flex flex-row space-x-2">
          {#if data.human.twitter}
            <a target="_blank" href={data.human.twitter}
              ><svg
                xmlns="http://www.w3.org/2000/svg"
                fill="currentColor"
                height="1.5em"
                viewBox="0 0 512 512"
                ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
                  d="M389.2 48h70.6L305.6 224.2 487 464H345L233.7 318.6 106.5 464H35.8L200.7 275.5 26.8 48H172.4L272.9 180.9 389.2 48zM364.4 421.8h39.1L151.1 88h-42L364.4 421.8z"
                /></svg
              ></a
            >
          {/if}
          {#if data.human.website}
            <a target="_blank" href={data.human.website}
              ><svg
                xmlns="http://www.w3.org/2000/svg"
                height="1.5em"
                fill="currentColor"
                viewBox="0 0 576 512"
                ><!--! Font Awesome Free 6.4.2 by @fontawesome - https://fontawesome.com License - https://fontawesome.com/license (Commercial License) Copyright 2023 Fonticons, Inc. --><path
                  d="M575.8 255.5c0 18-15 32.1-32 32.1h-32l.7 160.2c0 2.7-.2 5.4-.5 8.1V472c0 22.1-17.9 40-40 40H456c-1.1 0-2.2 0-3.3-.1c-1.4 .1-2.8 .1-4.2 .1H416 392c-22.1 0-40-17.9-40-40V448 384c0-17.7-14.3-32-32-32H256c-17.7 0-32 14.3-32 32v64 24c0 22.1-17.9 40-40 40H160 128.1c-1.5 0-3-.1-4.5-.2c-1.2 .1-2.4 .2-3.6 .2H104c-22.1 0-40-17.9-40-40V360c0-.9 0-1.9 .1-2.8V287.6H32c-18 0-32-14-32-32.1c0-9 3-17 10-24L266.4 8c7-7 15-8 22-8s15 2 21 7L564.8 231.5c8 7 12 15 11 24z"
                /></svg
              ></a
            >
          {/if}
        </ul>
      </td>
    </tr>
  </table>

  <!-- Description -->
  <div class="text-left px-4 py-4 space-y-4">
    {#if data.human.description}
      <SvelteMarkdown source={data.human.description} />
    {/if}
  </div>

  <!-- Tags -->
  <div class="py-4">
    <ul class="flex flex-row">
      {#if data.human.tags}
        {#each data.human.tags as tag}
          <li>
            <Chip><a href="/search?query={tag}">{tag}</a></Chip>
          </li>
        {/each}
      {/if}
    </ul>
  </div>

  <!-- Affiliate Links -->
  {#if data.human.affiliates.length > 0}
    <h2 class="mt-4 text-xl">Affiliate Links</h2>
    <div
      class="md:flex md:flex-row md:justify-between dark:text-white bg-gray-200 dark:bg-gray-800"
    >
      {#each data.human.affiliates as affiliate}
        <div class="max-w-md mx-auto my-2 p-6 rounded shadow-lg">
          <Affiliate
            image={affiliate.image}
            url={affiliate.url}
            name={affiliate.name}
          />
        </div>
      {/each}
    </div>
  {/if}
</article>
